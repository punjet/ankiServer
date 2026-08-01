package middleware

import (
	"net/http"
)

// SecurityHeaders adds hardened HTTP security headers to every response.
//
// These are the recommended headers for any publicly-exposed JSON API:
//   - X-Content-Type-Options: prevents MIME-sniffing attacks
//   - X-Frame-Options: prevents clickjacking (not critical for an API but harmless)
//   - X-XSS-Protection: legacy header, still respected by some older proxies
//   - Referrer-Policy: no referrer leaked from API responses
//   - Permissions-Policy: disables browser features not needed by an API
//   - Content-Security-Policy: strict policy — this is a JSON API, not a browser app
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'none'")
		// Remove headers that leak server info.
		h.Del("Server")
		h.Del("X-Powered-By")
		next.ServeHTTP(w, r)
	})
}

// MaxBodySize limits the request body to prevent abuse / memory exhaustion.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
