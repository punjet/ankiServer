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

func TestConfigHostAndDeviceKeys(t *testing.T) {
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("API_KEY", "app")
	t.Setenv("API_KEYS", " phone , ,laptop")
	t.Setenv("CONFIG_WRITABLE", "false")
	t.Setenv("MAX_BUFFER_NOTES", "50")

	cfg := Load()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("host: %q", cfg.Host)
	}
	got := cfg.AllAPIKeys()
	if len(got) != 3 || got[0] != "app" || got[1] != "phone" || got[2] != "laptop" {
		t.Errorf("keys: %v", got)
	}
	if cfg.ConfigWritable {
		t.Error("config should be read-only")
	}
	if cfg.MaxBufferNotes != 50 {
		t.Errorf("max buffer: %d", cfg.MaxBufferNotes)
	}
}

func TestConfigSecureDefaults(t *testing.T) {
	for _, e := range []string{"HOST", "API_KEY", "API_KEYS", "AUTH_FAIL_LIMIT", "CONFIG_WRITABLE", "MAX_BUFFER_NOTES", "ALLOW_NO_AUTH"} {
		t.Setenv(e, "")
	}
	cfg := Load()
	if cfg.Host != "" || len(cfg.AllAPIKeys()) != 0 || cfg.AuthFailLimit != 10 || cfg.MaxBufferNotes != 5000 ||
		cfg.AllowNoAuth || cfg.ConfigWritable {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}
