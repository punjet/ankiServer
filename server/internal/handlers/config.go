package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	aiClient "github.com/punjet/ankiserver/internal/ai"
	"github.com/punjet/ankiserver/internal/deepl"
)

// DeepLKeyStore is a simple thread-safe in-memory store for the DeepL API key.
type DeepLKeyStore struct {
	mu     sync.RWMutex
	key    string
	client *deepl.Client
}

// NewDeepLKeyStore creates a store pre-seeded with an optional key.
func NewDeepLKeyStore(initialKey string) *DeepLKeyStore {
	s := &DeepLKeyStore{}
	if initialKey != "" {
		s.set(initialKey)
	}
	return s
}

// Get returns the current DeepL client (nil if no key is set).
func (s *DeepLKeyStore) Get() *deepl.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

// HasKey returns true when a key is configured.
func (s *DeepLKeyStore) HasKey() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.key != ""
}

func (s *DeepLKeyStore) set(key string) {
	s.key = key
	s.client = deepl.New(key)
}

// Config handles GET/POST /config.
//
// GET  — returns server status (DeepL key presence, OpenAI key presence, monthly char count).
// POST — allows setting keys at runtime without restart.
//
// setOpenAIKey is a callback provided by main to update the ai.Client store.
func Config(
	deepLStore *DeepLKeyStore,
	getAI func() *aiClient.Client,
	setOpenAIKey func(key string),
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body struct {
				DeepLKey  string `json:"deepl_key"`
				OpenAIKey string `json:"openai_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
				return
			}

			if body.DeepLKey != "" {
				deepLStore.mu.Lock()
				deepLStore.set(body.DeepLKey)
				deepLStore.mu.Unlock()
				logger.Info("🔑 DeepL key updated via API")
			}

			if body.OpenAIKey != "" {
				setOpenAIKey(body.OpenAIKey)
				logger.Info("🤖 OpenAI key updated via API")
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}

		// GET — return current status.
		month, countStr := CharStats()
		count, _ := strconv.Atoi(countStr)

		writeJSON(w, http.StatusOK, map[string]any{
			"has_deepl_key":          deepLStore.HasKey(),
			"has_openai_key":         getAI() != nil,
			"deepl_chars_this_month": count,
			"deepl_chars_month":      month,
		})
	}
}
