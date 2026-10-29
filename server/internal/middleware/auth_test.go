package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func do(h http.Handler, path, key, ip string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

func TestAuthAcceptsAnyDeviceKey(t *testing.T) {
	h := Auth([]string{"app-key", "iphone-key"}, 10)(ok)
	for _, key := range []string{"app-key", "iphone-key"} {
		if code := do(h, "/buffer", key, "10.0.0.1"); code != http.StatusOK {
			t.Errorf("key %q: got %d", key, code)
		}
	}
	if code := do(h, "/buffer", "revoked", "10.0.0.1"); code != http.StatusUnauthorized {
		t.Errorf("unknown key: got %d", code)
	}
	if code := do(h, "/buffer", "", "10.0.0.1"); code != http.StatusUnauthorized {
		t.Errorf("missing key: got %d", code)
	}
}

func TestAuthBearerHeader(t *testing.T) {
	h := Auth([]string{"k"}, 10)(ok)
	req := httptest.NewRequest(http.MethodGet, "/buffer", nil)
	req.Header.Set("Authorization", "Bearer k")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got %d", rec.Code)
	}
}

func TestAuthLocksOutAfterRepeatedFailures(t *testing.T) {
	h := Auth([]string{"good"}, 3)(ok)
	for i := 0; i < 3; i++ {
		if code := do(h, "/buffer", "bad", "10.0.0.2"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: got %d", i, code)
		}
	}
	if code := do(h, "/buffer", "good", "10.0.0.2"); code != http.StatusTooManyRequests {
		t.Errorf("locked-out IP must be refused even with the right key, got %d", code)
	}
	if code := do(h, "/buffer", "good", "10.0.0.3"); code != http.StatusOK {
		t.Errorf("other IPs are unaffected, got %d", code)
	}
}

func TestAuthSuccessesDontCountTowardsLockout(t *testing.T) {
	h := Auth([]string{"good"}, 2)(ok)
	for i := 0; i < 20; i++ {
		if code := do(h, "/buffer", "good", "10.0.0.4"); code != http.StatusOK {
			t.Fatalf("request %d: got %d", i, code)
		}
	}
}

func TestAuthHealthIsPublicAndDisabledWithoutKeys(t *testing.T) {
	h := Auth([]string{"k"}, 1)(ok)
	if code := do(h, "/health", "", "10.0.0.5"); code != http.StatusOK {
		t.Errorf("/health: got %d", code)
	}
	if code := do(Auth(nil, 1)(ok), "/buffer", "", "10.0.0.5"); code != http.StatusOK {
		t.Errorf("no keys configured: got %d", code)
	}
}
