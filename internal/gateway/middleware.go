package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func Logger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("request processed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	limiters := make(map[string]*rate.Limiter)
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr // Simplified, should use RealIP
			mu.Lock()
			limiter, ok := limiters[ip]
			if !ok {
				limiter = rate.NewLimiter(rate.Limit(rps), burst)
				limiters[ip] = limiter
			}
			mu.Unlock()

			if !limiter.Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CircuitBreaker(next http.Handler) http.Handler {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "gateway",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     10 * time.Second,
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := cb.Execute(func() (interface{}, error) {
			next.ServeHTTP(w, r)
			return nil, nil // Note: this doesn't capture HTTP error codes easily
		})
		if err != nil {
			http.Error(w, "service unavailable (circuit open)", http.StatusServiceUnavailable)
		}
	})
}

func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
