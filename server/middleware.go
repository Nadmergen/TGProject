package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"golang.org/x/time/rate"
)

type Middleware struct {
	db      *sql.DB
	limiter *rate.Limiter
}

type ctxKey string

const ctxKeyUserID ctxKey = "userID"

func NewMiddleware(db *sql.DB) *Middleware {
	return &Middleware{
		db:      db,
		limiter: rate.NewLimiter(rate.Limit(100), 10), // 100 requests/sec, burst of 10
	}
}

// ==================== SECURE CORS ====================
func (m *Middleware) SecureCORS(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"https://yourdomain.com": true,
		"http://localhost:3000":  true,
		"tauri://localhost":      true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ==================== RATE LIMITING ====================
func (m *Middleware) RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.limiter.Allow() {
			Slog.Warn("rate limit exceeded", slog.String("ip", r.RemoteAddr))
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"Too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ==================== SECURITY HEADERS ====================
func (m *Middleware) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// ==================== AUTH MIDDLEWARE ====================
func (m *Middleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			token = authHeader
		}

		var userID int64
		var username string
		err := m.db.QueryRow(
			"SELECT user_id, COALESCE((SELECT username FROM users WHERE id = user_id), '') FROM sessions WHERE token = $1 AND expires_at > NOW()",
			token,
		).Scan(&userID, &username)

		if err == sql.ErrNoRows {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			return
		}

		if err != nil {
			Slog.Error("auth query failed", slog.Any("error", err))
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
			return
		}

		if userID <= 0 {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid user"})
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
		r = r.WithContext(ctx)
		r.Header.Set("X-User-ID", strconv.FormatInt(userID, 10))

		next.ServeHTTP(w, r)
	})
}

func getUserID(r *http.Request) (int64, bool) {
	if v := r.Context().Value(ctxKeyUserID); v != nil {
		if id, ok := v.(int64); ok && id > 0 {
			return id, true
		}
	}
	if s := strings.TrimSpace(r.Header.Get("X-User-ID")); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// ==================== REQUEST SIZE LIMIT ====================
func (m *Middleware) MaxBodySize(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			next.ServeHTTP(w, r)
		})
	}
}

// ==================== LOGGING ====================
func (m *Middleware) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		Slog.Info("request started",
			Slog.String("method", r.Method),
			Slog.String("path", r.RequestURI),
			Slog.String("ip", r.RemoteAddr),
		)
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		Slog.Info("request completed",
			Slog.String("method", r.Method),
			Slog.String("path", r.RequestURI),
			Slog.Duration("duration", duration),
		)
	})
}

// ==================== REQUEST ID ====================
func (m *Middleware) RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateToken()
		ctx := context.WithValue(r.Context(), "requestID", requestID)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}