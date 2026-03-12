package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db    *sql.DB
	cache *redis.Client
	email *SMTPEmailService
}

type AuthResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Token       string `json:"token,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
	RequiresOTP bool   `json:"requires_otp,omitempty"`
}

func NewAuthService(db *sql.DB, cache *redis.Client) *AuthService {
	return &AuthService{
		db:    db,
		cache: cache,
		email: NewSMTPEmailService(),
	}
}

// === ИНИЦИАЦИЯ РЕГИСТРАЦИИ (ТОЛЬКО Email) ===
func (as *AuthService) InitRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{Status: "error", Message: "Method not allowed"})
		return
	}

	var req struct {
		Email string `json:\"email\"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return
	}

	// Генерация OTP
	otp := generateOTP()
	ctx := context.Background()

	// Сохраняем OTP в Redis на 5 минут
	err := as.cache.Set(ctx, fmt.Sprintf("reg_otp:%s", req.Email), otp, 5*time.Minute).Err()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to save OTP"})
		return
	}

	// Отправка Email (в мок-режиме просто выведет в консоль)
	as.email.SendOTP(req.Email, otp)

	log.Printf("📩 OTP for %s: %s", req.Email, otp)

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Verification code sent to email",
	})
}

// === ЗАВЕРШЕНИЕ РЕГИСТРАЦИИ ===
func (as *AuthService) CompleteRegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:\"email\"`
		Username string `json:\"username\"`
		Password string `json:\"password\"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	var userID int
	err := as.db.QueryRow(
		"INSERT INTO users (username, email, password, phone) VALUES ($1, $2, $3, $4) RETURNING id",
		req.Username, req.Email, string(hashedPassword), "", // Передаем пустую строку вместо телефона
	).Scan(&userID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "User creation failed"})
		return
	}

	respondJSON(w, http.StatusOK, AuthResponse{Status: "success", Message: "Registration complete", UserID: userID})
}

// === ПРОВЕРКА КОДА И РЕГИСТРАЦИЯ ===
func (as *AuthService) VerifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{
			Status:  "error",
			Message: "Method not allowed",
		})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
		Phone    string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, AuthResponse{
			Status:  "error",
			Message: "Invalid request",
		})
		return
	}

	// Проверяем код
	ctx := context.Background()
	storedCode, err := as.cache.Get(ctx, fmt.Sprintf("verify_code:%s", req.Email)).Result()
	if err != nil || storedCode != req.Code {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{
			Status:  "error",
			Message: "Invalid or expired verification code",
		})
		return
	}

	// Удаляем код из кеша
	as.cache.Del(ctx, fmt.Sprintf("verify_code:%s", req.Email))

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{
			Status:  "error",
			Message: "Password hashing failed",
		})
		return
	}

	// Создаем пользователя
	var userID int64
	err = as.db.QueryRow(
		"INSERT INTO users (username, email, phone, password_hash, is_verified, created_at, updated_at) VALUES ($1, $2, $3, $4, true, NOW(), NOW()) RETURNING id",
		req.Username, req.Email, req.Phone, string(hashedPassword),
	).Scan(&userID)

	if err != nil {
		log.Printf("❌ Registration error: %v\n", err)
		respondJSON(w, http.StatusBadRequest, AuthResponse{
			Status:  "error",
			Message: "Username already exists or registration failed",
		})
		return
	}

	// Создаем сессию
	token := generateToken()
	_, err = as.db.Exec(
		"INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, token, time.Now().Add(30*24*time.Hour),
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{
			Status:  "error",
			Message: "Session creation failed",
		})
		return
	}

	log.Printf("✅ User registered successfully: %s (ID: %d)\n", req.Username, userID)

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Registration successful!",
		Token:   token,
		UserID:  int(userID),
	})
}

// === ВХОД ===
func (as *AuthService) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{
			Status:  "error",
			Message: "Method not allowed",
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, AuthResponse{
			Status:  "error",
			Message: "Invalid request",
		})
		return
	}

	var userID int
	var passwordHash string
	var has2FA bool

	err := as.db.QueryRow(
		"SELECT id, password_hash, has_2fa FROM users WHERE username = $1",
		req.Username,
	).Scan(&userID, &passwordHash, &has2FA)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{
			Status:  "error",
			Message: "User not found",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{
			Status:  "error",
			Message: "Invalid password",
		})
		return
	}

	// Если включена 2FA
	if has2FA {
		code := generateOTP()
		ctx := context.Background()
		as.cache.Set(ctx, fmt.Sprintf("2fa_code:%d", userID), code, 5*time.Minute)

		// Отправляем код на email
		var email string
		as.db.QueryRow("SELECT email FROM users WHERE id = $1", userID).Scan(&email)
		as.email.SendVerificationEmail(email, code, "Your 2FA Code")

		log.Printf("✅ 2FA code sent to user %d\n", userID)

		respondJSON(w, http.StatusOK, AuthResponse{
			Status:      "success",
			Message:     "2FA code sent to your email",
			RequiresOTP: true,
			UserID:      userID,
		})
		return
	}

	// Обычный вход
	token := generateToken()
	_, err = as.db.Exec(
		"INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, token, time.Now().Add(30*24*time.Hour),
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{
			Status:  "error",
			Message: "Session creation failed",
		})
		return
	}

	log.Printf("✅ User logged in: ID %d\n", userID)

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Login successful",
		Token:   token,
		UserID:  userID,
	})
}

// === ПРОВЕРКА 2FA ===
func (as *AuthService) Verify2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{
			Status:  "error",
			Message: "Method not allowed",
		})
		return
	}

	var req struct {
		UserID int    `json:"user_id"`
		Code   string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, AuthResponse{
			Status:  "error",
			Message: "Invalid request",
		})
		return
	}

	ctx := context.Background()
	storedCode, err := as.cache.Get(ctx, fmt.Sprintf("2fa_code:%d", req.UserID)).Result()
	if err != nil || storedCode != req.Code {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{
			Status:  "error",
			Message: "Invalid 2FA code",
		})
		return
	}

	as.cache.Del(ctx, fmt.Sprintf("2fa_code:%d", req.UserID))

	token := generateToken()
	_, err = as.db.Exec(
		"INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)",
		req.UserID, token, time.Now().Add(30*24*time.Hour),
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{
			Status:  "error",
			Message: "Session creation failed",
		})
		return
	}

	log.Printf("✅ 2FA verified for user %d\n", req.UserID)

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "2FA verified",
		Token:   token,
		UserID:  req.UserID,
	})
}

// === LOGOUT ===
func (as *AuthService) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{
			Status:  "error",
			Message: "Method not allowed",
		})
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		respondJSON(w, http.StatusBadRequest, AuthResponse{
			Status:  "error",
			Message: "Token required",
		})
		return
	}

	// Удаляем сессию
	as.db.Exec("DELETE FROM sessions WHERE token = $1", authHeader)

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Logout successful",
	})
}
