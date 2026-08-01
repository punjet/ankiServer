package config

import (
	"os"
	"testing"
)

func TestConfigLoadDefaults(t *testing.T) {
	// Unset everything to ensure defaults trigger
	envs := []string{
		"PORT", "ANKI_URL", "DEEPL_KEY", "BUFFER_FILE", "LOG_LEVEL",
		"OPENAI_API_KEY", "OPENAI_MODEL", "MAX_CARDS_PER_REQUEST",
		"GRAMMAR_DECK_NAME", "GRAMMAR_MODEL_NAME", "API_KEY",
		"RATE_LIMIT_DEFAULT", "RATE_LIMIT_GRAMMAR", "MAX_BODY_BYTES",
		"MAX_TEXT_LENGTH", "TRUSTED_PROXIES",
	}
	
	for _, e := range envs {
		os.Unsetenv(e)
	}

	cfg := Load()

	if cfg.Port != "5005" {
		t.Errorf("expected 5005, got %s", cfg.Port)
	}
	if cfg.MaxCardsPerRequest != 5 {
		t.Errorf("expected 5, got %d", cfg.MaxCardsPerRequest)
	}
	if !cfg.TrustedProxies {
		t.Errorf("expected true, got %v", cfg.TrustedProxies)
	}
}

func TestConfigLoadOverrides(t *testing.T) {
	os.Setenv("PORT", "8080")
	os.Setenv("MAX_CARDS_PER_REQUEST", "10")
	os.Setenv("TRUSTED_PROXIES", "false")
	
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("MAX_CARDS_PER_REQUEST")
	defer os.Unsetenv("TRUSTED_PROXIES")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected 8080, got %s", cfg.Port)
	}
	if cfg.MaxCardsPerRequest != 10 {
		t.Errorf("expected 10, got %d", cfg.MaxCardsPerRequest)
	}
	if cfg.TrustedProxies {
		t.Errorf("expected false, got %v", cfg.TrustedProxies)
	}
}
