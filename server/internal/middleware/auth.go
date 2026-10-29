package middleware

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
)

// Auth returns a middleware that enforces API key authentication.
//
// The key is accepted in two forms (checked in order):
//  1. X-API-Key: <key>           header
//  2. Authorization: Bearer <key> header
//
// Any of `keys` is accepted (one per device). If keys is empty the middleware is
// a no-op — all requests pass through, for trusted local environments.
//
// An IP that fails `failLimit` times within a minute gets 429 until the window
// passes, even with a correct key, so keys can't be guessed at network speed.
//
// Timing-safe comparison (crypto/subtle) is used to prevent timing attacks.
func Auth(keys []string, failLimit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(keys) == 0 {
			return next
		}
		expected := make([][]byte, len(keys))
		for i, k := range keys {
			expected[i] = []byte(k)
		}
		failures := newStore(max(1, failLimit), 60)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// /health is always public — needed for Docker/Coolify health checks.
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			ip := realIP(r)
			attempts := failures.get(ip)
			if attempts.full() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"too_many_auth_failures","detail":"too many invalid API keys — retry after 60s"}`))
				return
			}

			provided := extractKey(r)
			if provided == "" {
				attempts.allow()
				log.Printf("Auth failed: missing API key from IP %s", ip)
				writeAuthError(w, "missing API key — provide X-API-Key header or Authorization: Bearer <key>")
				return
			}

			if !matches(provided, expected) {
				attempts.allow()
				log.Printf("Auth failed: invalid API key from IP %s", ip)
				writeAuthError(w, "invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// matches compares against every key so the timing doesn't reveal which one (if any) matched.
func matches(provided string, expected [][]byte) bool {
	p := []byte(provided)
	ok := 0
	for _, e := range expected {
		ok |= subtle.ConstantTimeCompare(p, e)
	}
	return ok == 1
}

// extractKey reads the API key from request headers.
func extractKey(r *http.Request) string {
	// Try X-API-Key first.
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	// Try Authorization: Bearer <token>.
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="anki-server"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","detail":"` + msg + `"}`))
}
