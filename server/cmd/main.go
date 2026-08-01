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
	wordModelName    = "WordsFromSafari"
	wordDeckName     = "WordsFromSafari"
)

var wordRequiredFields = []string{
	"Word", "WordTranslation", "Context", "ContextTranslation",
	"Audio", "Spelling", "SpellingTranscript", "SourceURL", "DateAdded", "SeenCount",
}

// aiStore is a thread-safe holder for the OpenAI client (key can be updated at runtime).
type aiStore struct {
	mu     sync.RWMutex
	client *aiClient.Client
}

func (s *aiStore) get() *aiClient.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *aiStore) set(key, model string, maxCards int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key != "" {
		s.client = aiClient.New(key, model, maxCards)
	} else {
		s.client = nil
	}
}

func main() {
	cfg := config.Load()

	// --- Logger ---
	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// --- Buffer ---
	buf, err := buffer.New(cfg.BufferFile)
	if err != nil {
		logger.Error("Failed to initialize buffer", "err", err)
		os.Exit(1)
	}
	logger.Info("💾 Buffer ready", "file", cfg.BufferFile)

	// --- Anki client ---
	ankiCl := anki.New(cfg.AnkiURL)
	logger.Info("🃏 AnkiConnect client ready", "url", cfg.AnkiURL)

	// Verify model fields in the background (Anki might not be open yet).
	go func() {
		time.Sleep(2 * time.Second)
		if err := ankiCl.EnsureModelFields(wordModelName, wordRequiredFields); err != nil {
			logger.Warn("⚠️  EnsureModelFields (word model) — Anki not running?", "err", err)
		} else {
			logger.Info("✅ Word model fields verified")
		}
		if err := ankiCl.EnsureModelFields(cfg.GrammarModelName, grammar.RequiredFields()); err != nil {
			logger.Warn("⚠️  EnsureModelFields (grammar model) — Anki not running?", "err", err)
		} else {
			logger.Info("✅ Grammar model fields verified")
		}
	}()

	// --- DeepL key store ---
	deepLStore := handlers.NewDeepLKeyStore(cfg.DeepLKey)
	if cfg.DeepLKey != "" {
		logger.Info("🔑 DeepL key loaded from environment")
	} else {
		logger.Warn("⚠️  DEEPL_KEY not set — /translate will error until configured")
	}

	// --- AI (OpenAI) store ---
	ai := &aiStore{}
	ai.set(cfg.OpenAIKey, cfg.OpenAIModel, cfg.MaxCardsPerRequest)
	if cfg.OpenAIKey != "" {
		logger.Info("🤖 OpenAI client ready", "model", cfg.OpenAIModel)
	} else {
		logger.Warn("⚠️  OPENAI_API_KEY not set — /grammar will error until configured")
	}

	// --- Grammar analyzer ---
	grammarAnalyzer := grammar.New(cfg.GrammarDeckName, cfg.GrammarModelName)

	// --- Router ---
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(middleware.CORS)

	// Vocabulary / word endpoints.
	r.Post("/add", handlers.Add(buf, logger))
	r.Post("/sync", handlers.Sync(buf, ankiCl, logger))
	r.Post("/translate", handlers.Translate(deepLStore.Get, logger))
	r.Post("/check", handlers.Check(ankiCl, wordDeckName, logger))

	// Grammar analysis endpoint.
	r.Post("/grammar", handlers.Grammar(buf, ai.get, grammarAnalyzer, logger))

	// Config endpoint (supports updating DeepL + OpenAI keys at runtime).
	r.Get("/config", handlers.Config(deepLStore, ai.get, func(key string) {
		ai.set(key, cfg.OpenAIModel, cfg.MaxCardsPerRequest)
	}, logger))
	r.Post("/config", handlers.Config(deepLStore, ai.get, func(key string) {
		ai.set(key, cfg.OpenAIModel, cfg.MaxCardsPerRequest)
	}, logger))

	// Health check for Docker/Coolify.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// --- HTTP Server with graceful shutdown ---
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 90 * time.Second, // longer for AI calls
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("🛑 Shutdown signal received")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Shutdown error", "err", err)
		}
		close(done)
	}()

	logger.Info("🚀 Anki server started", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error", "err", err)
		os.Exit(1)
	}
	<-done
	logger.Info("👋 Server stopped")
}
