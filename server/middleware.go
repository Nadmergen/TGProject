package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type Middleware struct {
	db *sql.DB
}

func NewMiddleware(db *sql.DB) *Middleware {
	return &Middleware{db: db}
}

// AuthRequired проверяет токен и добавляет userID в контекст
func (mw *Middleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		token := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			token = authHeader
		}

		if token == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}

		var userID int64
		err := mw.db.QueryRow(
			"SELECT user_id FROM sessions WHERE token = $1 AND expires_at > $2",
			token, time.Now(),
		).Scan(&userID)

		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
			return
		}

		// Добавляем ID пользователя в контекст
		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}