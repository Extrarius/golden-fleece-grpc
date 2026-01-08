package middleware

import (
	"log"
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimit ограничивает количество запросов (rate limiting)
func RateLimit(next http.Handler) http.Handler {
	// 100 запросов в секунду, burst 10 (разрешает кратковременные всплески)
	limiter := rate.NewLimiter(rate.Limit(100), 10)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			log.Printf("[HTTP] Rate limit exceeded for %s from %s", r.URL.Path, r.RemoteAddr)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
