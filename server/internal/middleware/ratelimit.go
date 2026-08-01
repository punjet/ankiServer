package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// bucket is a simple sliding-window counter for one IP.
type bucket struct {
	mu        sync.Mutex
	requests  []time.Time
	windowSec int
	limit     int
}

func newBucket(limit, windowSec int) *bucket {
	return &bucket{limit: limit, windowSec: windowSec}
}

// allow returns true if the request is within the rate limit.
func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(b.windowSec) * time.Second)

	// Evict old entries.
	j := 0
	for _, t := range b.requests {
		if t.After(cutoff) {
			b.requests[j] = t
			j++
		}
	}
	b.requests = b.requests[:j]

	if len(b.requests) >= b.limit {
		return false
	}
	b.requests = append(b.requests, now)
	return true
}

// store holds per-IP buckets.
type store struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	// Each bucket's limit / window (shared for all IPs in this store).
	limit     int
	windowSec int
}

func newStore(limit, windowSec int) *store {
	s := &store{
		buckets:   make(map[string]*bucket),
		limit:     limit,
		windowSec: windowSec,
	}
	// Periodic cleanup of idle buckets.
	go s.cleanup()
	return s
}

func (s *store) get(ip string) *bucket {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.buckets[ip]
	if !ok {
		b = newBucket(s.limit, s.windowSec)
		s.buckets[ip] = b
	}
	return b
}

func (s *store) cleanup() {
	for range time.Tick(5 * time.Minute) {
		s.mu.Lock()
		cutoff := time.Now().Add(-time.Duration(s.windowSec) * time.Second)
		for ip, b := range s.buckets {
			b.mu.Lock()
			empty := len(b.requests) == 0 || b.requests[len(b.requests)-1].Before(cutoff)
			b.mu.Unlock()
			if empty {
				delete(s.buckets, ip)
			}
		}
		s.mu.Unlock()
	}
}

// RateLimiter returns a middleware that limits requests per IP per minute.
//
//   - limit     — max requests allowed in the window
//   - windowSec — size of the sliding window in seconds
func RateLimiter(limit, windowSec int) func(http.Handler) http.Handler {
	s := newStore(limit, windowSec)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !s.get(ip).allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate_limit_exceeded","detail":"too many requests — slow down and retry after 60s"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realIP extracts the client IP, respecting X-Real-IP / X-Forwarded-For set by
// a trusted reverse proxy (chi's RealIP middleware populates r.RemoteAddr).
func realIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
