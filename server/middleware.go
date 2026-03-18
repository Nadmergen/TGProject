package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Middleware struct {
	db *sql.DB
}

type ctxKey string

const ctxKeyUserID ctxKey = "userID"

func NewMiddleware(db *sql.DB) *Middleware {
	return &Middleware{db: db}
}

// === MIDDLEWARE: Проверка авторизации ===
func (m *Middleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Извлекаем токен (формат: "Bearer token")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			// Если нет "Bearer", ищем сам токен
			token = authHeader
		}

		// Проверяем токен в БД
		var userID int

		err := m.db.QueryRow(
			"SELECT user_id FROM sessions WHERE token = $1 AND expires_at > NOW()",
			token,
		).Scan(&userID)

		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		if err != nil {
			http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
			return
		}

		// Добавляем userID в контекст
		ctx := context.WithValue(r.Context(), ctxKeyUserID, int64(userID))
		r = r.WithContext(ctx)

		// Добавляем userID в заголовок для других handlers
		r.Header.Set("X-User-ID", strconv.Itoa(userID))

		next.ServeHTTP(w, r)
	})
}

func getUserID(r *http.Request) (int64, bool) {
	if v := r.Context().Value(ctxKeyUserID); v != nil {
		if id, ok := v.(int64); ok && id > 0 {
			return id, true
		}
	}
	// Fallback (legacy) for handlers still using header.
	if s := strings.TrimSpace(r.Header.Get("X-User-ID")); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// === MIDDLEWARE: CORS ===
func (m *Middleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// === MIDDLEWARE: Логирование ===
func (m *Middleware) Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		method := r.Method
		path := r.RequestURI

		next.ServeHTTP(w, r)

		duration := time.Since(start)
		log.Printf("📝 %s %s - %dms\n", method, path, duration.Milliseconds())
	})
}

// === MIDDLEWARE: Request ID ===
func (m *Middleware) RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateToken()
		ctx := context.WithValue(r.Context(), "requestID", requestID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}
