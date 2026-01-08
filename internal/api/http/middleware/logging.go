package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseWriter обертка для ResponseWriter для логирования статуса ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Logging логирует все HTTP запросы с информацией о времени выполнения
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Логирование запроса
		log.Printf("[HTTP] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Обертка ResponseWriter для логирования статуса
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		// Логирование ответа
		log.Printf("[HTTP] %s %s - %d - %v", r.Method, r.URL.Path, ww.statusCode, time.Since(start))
	})
}
