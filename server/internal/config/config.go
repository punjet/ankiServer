package config

import (
	"fmt"
	"os"
)

// Config holds all server configuration loaded from environment variables.
type Config struct {
	// Port the HTTP server listens on.
	Port string

	// AnkiURL is the URL of the local AnkiConnect add-on.
	AnkiURL string

	// DeepLKey is the DeepL API authentication key.
	DeepLKey string

	// BufferFile is the path where the JSON card buffer is persisted.
	BufferFile string

	// LogLevel controls log verbosity: "debug", "info", "warn", "error".
	LogLevel string

	// OpenAIKey is the OpenAI API authentication key.
	OpenAIKey string

	// OpenAIModel is the model used for grammar analysis.
	OpenAIModel string

	// MaxCardsPerRequest limits how many Anki cards can be generated per /grammar call.
	MaxCardsPerRequest int

	// GrammarDeckName is the Anki deck for grammar error cards.
	GrammarDeckName string

	// GrammarModelName is the Anki note type for grammar cards.
	GrammarModelName string

	// ── Security ──────────────────────────────────────────────────────────────

	// APIKey is the server's own secret key that clients must send in
	// the X-API-Key header (or Authorization: Bearer <key>).
	// If empty, authentication is disabled (useful for local-only setups).
	APIKey string

	// RateLimitDefault is the max requests per minute per IP for general endpoints.
	RateLimitDefault int

	// RateLimitGrammar is the max requests per minute per IP for /grammar
	// (AI calls are expensive, so a tighter limit is applied).
	RateLimitGrammar int

	// MaxBodyBytes is the maximum allowed request body size in bytes.
	MaxBodyBytes int64

	// MaxTextLength is the maximum allowed length of the "text" field in /grammar.
	MaxTextLength int

	// TrustedProxies controls whether X-Forwarded-For is trusted for real IP.
	// Set to true only when behind a known reverse proxy (Coolify, nginx, etc.).
	TrustedProxies bool
}

// Load reads configuration from environment variables and applies defaults.
func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "5005"),
		AnkiURL:            getEnv("ANKI_URL", "http://host.docker.internal:8765"),
		DeepLKey:           getEnv("DEEPL_KEY", ""),
		BufferFile:         getEnv("BUFFER_FILE", "/data/anki_buffer.json"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		OpenAIKey:          getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:        getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		MaxCardsPerRequest: getEnvInt("MAX_CARDS_PER_REQUEST", 5),
		GrammarDeckName:    getEnv("GRAMMAR_DECK_NAME", "GrammarErrors"),
		GrammarModelName:   getEnv("GRAMMAR_MODEL_NAME", "GrammarErrors"),

		// Security
		APIKey:           getEnv("API_KEY", ""),
		RateLimitDefault: getEnvInt("RATE_LIMIT_DEFAULT", 60),
		RateLimitGrammar: getEnvInt("RATE_LIMIT_GRAMMAR", 10),
		MaxBodyBytes:     int64(getEnvInt("MAX_BODY_BYTES", 65536)), // 64 KB
		MaxTextLength:    getEnvInt("MAX_TEXT_LENGTH", 4000),
		TrustedProxies:   getEnvBool("TRUSTED_PROXIES", true),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
}
