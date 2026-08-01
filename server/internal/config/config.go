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
