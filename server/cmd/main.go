package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/punjet/ankiserver/internal/anki"
	"github.com/punjet/ankiserver/internal/buffer"
	"github.com/punjet/ankiserver/internal/config"
	"github.com/punjet/ankiserver/internal/handlers"
	"github.com/punjet/ankiserver/internal/middleware"
)

const (
	modelName = "WordsFromSafari"
	deckName  = "WordsFromSafari"
)

var requiredFields = []string{
	"Word", "WordTranslation", "Context", "ContextTranslation",
	"Audio", "Spelling", "SpellingTranscript", "SourceURL", "DateAdded", "SeenCount",
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
	ankiClient := anki.New(cfg.AnkiURL)
	logger.Info("🃏 AnkiConnect client ready", "url", cfg.AnkiURL)

	// Try to ensure model fields (non-fatal if Anki is not running yet).
	go func() {
		time.Sleep(2 * time.Second)
		if err := ankiClient.EnsureModelFields(modelName, requiredFields); err != nil {
			logger.Warn("⚠️  EnsureModelFields (Anki not running?)", "err", err)
		} else {
			logger.Info("✅ Anki model fields verified")
		}
	}()

	// --- DeepL key store ---
	keyStore := handlers.NewDeepLKeyStore(cfg.DeepLKey)
	if cfg.DeepLKey != "" {
		logger.Info("🔑 DeepL key loaded from environment")
	} else {
		logger.Warn("⚠️  DEEPL_KEY not set — translation endpoint will return error until set via /config")
	}

	// --- Router ---
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(middleware.CORS)

	r.Post("/add", handlers.Add(buf, logger))
	r.Post("/sync", handlers.Sync(buf, ankiClient, logger))
	r.Post("/translate", handlers.Translate(keyStore.Get, logger))
	r.Post("/check", handlers.Check(ankiClient, deckName, logger))
	r.Get("/config", handlers.Config(keyStore, logger))
	r.Post("/config", handlers.Config(keyStore, logger))

	// Health check for Docker/Coolify.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// --- Server ---
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	done := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("🛑 Shutdown signal received")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
