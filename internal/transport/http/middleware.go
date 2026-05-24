package httptransport

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
		})
	}
}

func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	buckets := map[string]bucket{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			now := time.Now()
			mu.Lock()
			b := buckets[host]
			if now.After(b.reset) {
				b = bucket{reset: now.Add(window)}
			}
			b.count++
			buckets[host] = b
			mu.Unlock()
			if b.count > limit {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
