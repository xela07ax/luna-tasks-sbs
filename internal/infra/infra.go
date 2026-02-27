package infra

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"
)

// JWT
type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	ttl        time.Duration
}

func NewJWTManager(privKeyPEM, pubKeyPEM []byte, ttl time.Duration) (*JWTManager, error) {
	var privKey *rsa.PrivateKey
	var pubKey *rsa.PublicKey
	var err error

	if len(privKeyPEM) > 0 {
		privKey, err = jwt.ParseRSAPrivateKeyFromPEM(privKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	if len(pubKeyPEM) > 0 {
		pubKey, err = jwt.ParseRSAPublicKeyFromPEM(pubKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	return &JWTManager{privateKey: privKey, publicKey: pubKey, ttl: ttl}, nil
}

func (m *JWTManager) GenerateToken(userID int64, username string) (string, error) {
	if m.privateKey == nil {
		return "", errors.New("private key is not configured for token generation")
	}

	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(m.ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

func AuthMiddleware(pubKeyPEM []byte) func(http.Handler) http.Handler {
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyPEM)
	if err != nil {
		// В реальном приложении лучше падать при старте, но для middleware
		// мы просто будем отклонять все запросы, если ключ невалиден.
		panic(fmt.Sprintf("invalid public key for AuthMiddleware: %v", err))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return pubKey, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			userID := int64(claims["user_id"].(float64))
			ctx := context.WithValue(r.Context(), "user_id", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Rate Limiter
func RateLimitMiddleware(requestsPerMinute float64, burst int) func(http.Handler) http.Handler {
	var (
		limiters = make(map[int64]*rate.Limiter)
		mu       sync.Mutex
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value("user_id").(int64)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			mu.Lock()
			limiter, exists := limiters[userID]
			if !exists {
				limiter = rate.NewLimiter(rate.Limit(requestsPerMinute/60.0), burst)
				limiters[userID] = limiter
			}
			mu.Unlock()

			if !limiter.Allow() {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS Middleware
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Metrics
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})
	
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "HTTP request duration in seconds",
	}, []string{"method", "path"})
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		
		next.ServeHTTP(rw, r)
		
		duration := time.Since(start).Seconds()
		statusStr := "200" // Simplify for example
		if rw.status != 0 {
			statusStr = "error"
		}
		
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// Circuit Breaker (Simple implementation)
type CircuitBreaker struct {
	failureThreshold int
	failures         int
	lastFailure      time.Time
	mu               sync.Mutex
}

func NewCircuitBreaker(threshold int) *CircuitBreaker {
	return &CircuitBreaker{failureThreshold: threshold}
}

func (cb *CircuitBreaker) Execute(req func() error) error {
	cb.mu.Lock()
	if cb.failures >= cb.failureThreshold {
		if time.Since(cb.lastFailure) < 30*time.Second {
			cb.mu.Unlock()
			return errors.New("circuit breaker open")
		}
		cb.failures = 0 // Half-open
	}
	cb.mu.Unlock()

	err := req()

	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		return err
	}
	cb.failures = 0
	return nil
}

var EmailCircuitBreaker = NewCircuitBreaker(3)
