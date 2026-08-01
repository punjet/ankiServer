package config

import (
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
}

// Load reads configuration from environment variables and applies defaults.
func Load() *Config {
	return &Config{
		Port:       getEnv("PORT", "5005"),
		AnkiURL:    getEnv("ANKI_URL", "http://host.docker.internal:8765"),
		DeepLKey:   getEnv("DEEPL_KEY", ""),
		BufferFile: getEnv("BUFFER_FILE", "/data/anki_buffer.json"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
