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
// If apiKey is empty the middleware is a no-op — all requests pass through.
// This allows running without auth in trusted local environments.
//
// Timing-safe comparison (crypto/subtle) is used to prevent timing attacks.
func Auth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if apiKey == "" {
			// Auth disabled — pass all requests through.
			return next
		}

		expectedKey := []byte(apiKey)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// /health is always public — needed for Docker/Coolify health checks.
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			provided := extractKey(r)
			if provided == "" {
				log.Printf("Auth failed: missing API key from IP %s", r.RemoteAddr)
				writeAuthError(w, "missing API key — provide X-API-Key header or Authorization: Bearer <key>")
				return
			}

			// Constant-time comparison to prevent timing side-channel attacks.
			if subtle.ConstantTimeCompare([]byte(provided), expectedKey) != 1 {
				log.Printf("Auth failed: invalid API key from IP %s", r.RemoteAddr)
				writeAuthError(w, "invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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
