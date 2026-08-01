package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	aiClient "github.com/punjet/ankiserver/internal/ai"
	"github.com/punjet/ankiserver/internal/anki"
	"github.com/punjet/ankiserver/internal/buffer"
	"github.com/punjet/ankiserver/internal/config"
	"github.com/punjet/ankiserver/internal/grammar"
	"github.com/punjet/ankiserver/internal/handlers"
	"github.com/punjet/ankiserver/internal/middleware"
)

const (
	wordModelName = "WordsFromSafari"
	wordDeckName  = "WordsFromSafari"
)

var wordRequiredFields = []string{
	"Word", "WordTranslation", "Context", "ContextTranslation",
	"Audio", "Spelling", "SpellingTranscript", "SourceURL", "DateAdded", "SeenCount",
}

// aiStore is a thread-safe holder for the OpenAI client.
// The key can be updated at runtime via POST /config without restart.
type aiStore struct {
	mu     sync.RWMutex
	client *aiClient.Client
	model  string
	max    int
}

func (s *aiStore) get() *aiClient.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *aiStore) set(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key != "" {
		s.client = aiClient.New(key, s.model, s.max)
	} else {
		s.client = nil
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "5005"
		}
		resp, err := http.Get("http://localhost:" + port + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := config.Load()

	// ── Logger ────────────────────────────────────────────────────────────────
	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// ── Security startup warnings ─────────────────────────────────────────────
	if cfg.APIKey == "" {
		logger.Warn("⚠️  API_KEY not set — server is open without authentication! Set API_KEY in production.")
	} else {
		logger.Info("🔒 API key authentication enabled")
	}

	// ── Buffer ────────────────────────────────────────────────────────────────
	buf, err := buffer.New(cfg.BufferFile)
	if err != nil {
		logger.Error("Failed to initialize buffer", "err", err)
		os.Exit(1)
	}
	logger.Info("💾 Buffer ready", "file", cfg.BufferFile)

	// ── Anki client ───────────────────────────────────────────────────────────
	ankiCl := anki.New(cfg.AnkiURL)
	logger.Info("🃏 AnkiConnect client ready", "url", cfg.AnkiURL)

	// Verify/create model fields in the background (Anki may not be open yet).
	go func() {
		time.Sleep(3 * time.Second)
		if err := ankiCl.EnsureModelFields(wordModelName, wordRequiredFields); err != nil {
			logger.Warn("⚠️  EnsureModelFields (word model)", "err", err)
		} else {
			logger.Info("✅ Word model fields verified")
		}
		if err := ankiCl.EnsureModelFields(cfg.GrammarModelName, grammar.RequiredFields()); err != nil {
			logger.Warn("⚠️  EnsureModelFields (grammar model)", "err", err)
		} else {
			logger.Info("✅ Grammar model fields verified")
		}
	}()

	// ── DeepL key store ───────────────────────────────────────────────────────
	deepLStore := handlers.NewDeepLKeyStore(cfg.DeepLKey)
	if cfg.DeepLKey != "" {
		logger.Info("🔑 DeepL key loaded")
	} else {
		logger.Warn("⚠️  DEEPL_KEY not set — /translate requires it")
	}

	// ── AI (OpenAI) store ─────────────────────────────────────────────────────
	ai := &aiStore{model: cfg.OpenAIModel, max: cfg.MaxCardsPerRequest}
	ai.set(cfg.OpenAIKey)
	if cfg.OpenAIKey != "" {
		logger.Info("🤖 OpenAI client ready", "model", cfg.OpenAIModel)
	} else {
		logger.Warn("⚠️  OPENAI_API_KEY not set — /grammar requires it")
	}

	// ── Grammar analyzer ──────────────────────────────────────────────────────
	grammarAnalyzer := grammar.New(cfg.GrammarDeckName, cfg.GrammarModelName)

	// ── Router ────────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// ── Global middleware (applied to ALL routes) ─────────────────────────────
	// Order matters: security headers first, then body limit, then real IP,
	// then auth, then rate limiter, then logging/recovery last.
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.MaxBodySize(cfg.MaxBodyBytes))
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.RequestID)
	r.Use(middleware.Auth(cfg.APIKey))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Logger)
	r.Use(middleware.CORS)

	// ── Routes ────────────────────────────────────────────────────────────────

	// Health check — always public, no auth, no rate limit.
	// (Auth middleware explicitly bypasses /health)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","version":"2.0"}`)
	})

	// Standard endpoints — 60 req/min per IP (default).
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimiter(cfg.RateLimitDefault, 60))

		r.Post("/add", handlers.Add(buf, ai.get, logger))
		r.Post("/sync", handlers.Sync(buf, ankiCl, logger))
		r.Post("/translate", handlers.Translate(deepLStore.Get, logger))
		r.Post("/check", handlers.Check(ankiCl, wordDeckName, logger))
		r.Get("/config", handlers.Config(deepLStore, ai.get, ai.set, logger))
		r.Post("/config", handlers.Config(deepLStore, ai.get, ai.set, logger))
		r.Get("/buffer", handlers.GetBuffer(buf, logger))
		r.Delete("/buffer", handlers.DeleteBuffer(buf, logger))
	})

	// Grammar endpoint — tighter limit (10 req/min) because each request
	// costs an OpenAI API call.
	r.Group(func(r chi.Router) {
		r.Use(middleware.RateLimiter(cfg.RateLimitGrammar, 60))

		r.Post("/grammar", handlers.Grammar(buf, ai.get, grammarAnalyzer, cfg.MaxTextLength, logger))
	})

	// ── HTTP server ───────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
		// Timeouts prevent slow-loris and resource exhaustion attacks.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second, // generous for AI calls
		IdleTimeout:       60 * time.Second,
	}

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	done := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit
		logger.Info("🛑 Shutdown signal received", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Shutdown error", "err", err)
		}
		close(done)
	}()

	logger.Info("🚀 Anki server started", "addr", ":"+cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server listen error", "err", err)
		os.Exit(1)
	}
	<-done
	logger.Info("👋 Server stopped gracefully")
}
