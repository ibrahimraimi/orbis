package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"orbis/internal/discovery"
	"orbis/internal/observability"
)

type contextKey string

const ConsumerIDKey contextKey = "consumer_id"

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

func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "authorization header missing", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenStr := parts[1]
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func APIKeyAuth(resolver *discovery.Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				http.Error(w, "missing API key", http.StatusUnauthorized)
				return
			}

			hash := sha256.Sum256([]byte(apiKey))
			hashStr := hex.EncodeToString(hash[:])

			consumer, ok := resolver.GetConsumerByKeyHash(hashStr)
			if !ok {
				http.Error(w, "invalid API key", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ConsumerIDKey, consumer.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MetricsAndTracing(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			
			spanCtx, span := tracer.Start(ctx, r.URL.Path)
			defer span.End()

			consumerID := "anonymous"
			if val := r.Context().Value(ConsumerIDKey); val != nil {
				if id, ok := val.(string); ok {
					consumerID = id
					span.SetAttributes(attribute.String("consumer_id", id))
				}
			}

			start := time.Now()
			
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			next.ServeHTTP(rw, r.WithContext(spanCtx))
			
			duration := time.Since(start).Seconds()
			
			observability.GatewayRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", rw.statusCode), consumerID).Inc()
			observability.GatewayRequestLatency.WithLabelValues(r.Method, r.URL.Path, consumerID).Observe(duration)
		})
	}
}

func getRealIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

type rateLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	limiters := make(map[string]*rateLimiter)
	var mu sync.Mutex

	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, l := range limiters {
				if time.Since(l.lastSeen) > 5*time.Minute {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getRealIP(r)
			
			mu.Lock()
			l, ok := limiters[ip]
			if !ok {
				l = &rateLimiter{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
				limiters[ip] = l
			}
			l.lastSeen = time.Now()
			mu.Unlock()

			if !l.limiter.Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func CircuitBreaker(next http.Handler) http.Handler {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "gateway",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     10 * time.Second,
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		_, err := cb.Execute(func() (interface{}, error) {
			next.ServeHTTP(rw, r)
			if rw.statusCode >= 500 {
				return nil, fmt.Errorf("upstream server error: %d", rw.statusCode)
			}
			return nil, nil
		})
		
		if err != nil && rw.statusCode < 500 {
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
