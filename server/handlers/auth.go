package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB    *sql.DB
	Cache *redis.Client
}

// 1. Инициация регистрации (отправка кода)
func (h *AuthHandler) InitRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Генерируем 6-значный код
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Сохраняем в Redis на 5 минут
	ctx := context.Background()
	err := h.Cache.Set(ctx, "reg_code:"+req.Email, code, 5*time.Minute).Err()
	if err != nil {
		http.Error(w, "Redis error", http.StatusInternalServerError)
		return
	}

	// ТУТ ВСТАВИТЬ ВЫЗОВ ТВОЕЙ ФУНКЦИИ ОТПРАВКИ EMAIL
	fmt.Printf("📧 Код для %s: %s\n", req.Email, code)

	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Code sent"})
}

// 2. Завершение регистрации
func (h *AuthHandler) CompleteRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	storedCode, err := h.Cache.Get(ctx, "reg_code:"+req.Email).Result()
	if err != nil || storedCode != req.Code {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Неверный или просроченный код"})
		return
	}

	// Хешируем пароль
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	// Сохраняем в БД
	var userID int
	err = h.DB.QueryRow(
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		req.Username, req.Email, string(hashedPassword),
	).Scan(&userID)

	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Логин или Email уже занят"})
		return
	}

	// Удаляем код из Redis после успеха
	h.Cache.Del(ctx, "reg_code:"+req.Email)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"user_id": userID,
		"token":   "some-jwt-token", // Тут сгенерируй реальный токен
	})
}
