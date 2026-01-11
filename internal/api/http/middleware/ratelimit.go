package middleware

import (
	"log"
	"net/http"

	"golang.org/x/time/rate"
)

// RateLimit ограничивает количество запросов (rate limiting)
// rps - запросов в секунду, burst - разрешает кратковременные всплески
func RateLimit(next http.Handler, rps int, burst int) http.Handler {
	// Значения по умолчанию если не указаны
	if rps <= 0 {
		rps = 100
	}
	if burst <= 0 {
		burst = 10
	}

	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			log.Printf("[HTTP] Rate limit exceeded for %s from %s", r.URL.Path, r.RemoteAddr)
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
