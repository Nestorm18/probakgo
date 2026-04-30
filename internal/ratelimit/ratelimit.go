package ratelimit

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type bucket struct {
	count int
	reset time.Time
}

// Limiter is a fixed-window per-IP rate limiter.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	max     int
	window  time.Duration
}

func New(max int, window time.Duration) *Limiter {
	l := &Limiter{
		buckets: make(map[string]*bucket),
		max:     max,
		window:  window,
	}
	go l.cleanup()
	return l
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.window * 2)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		l.mu.Lock()
		for ip, b := range l.buckets {
			if now.After(b.reset) {
				delete(l.buckets, ip)
			}
		}
		l.mu.Unlock()
	}
}

func (l *Limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok || now.After(b.reset) {
		l.buckets[ip] = &bucket{count: 1, reset: now.Add(l.window)}
		return true
	}
	b.count++
	return b.count <= l.max
}

// Middleware returns a plain-text 429 when the limit is exceeded (for web UI).
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(ExtractIP(r)) {
			w.Header().Set("Retry-After", strconv.Itoa(int(l.window.Seconds())))
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// JSONMiddleware returns a JSON 429 when the limit is exceeded (for API).
func (l *Limiter) JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(ExtractIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(int(l.window.Seconds())))
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many requests"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ExtractIP gets the client IP from RemoteAddr. chi's RealIP middleware
// already rewrites RemoteAddr from X-Real-IP / X-Forwarded-For when present.
func ExtractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
