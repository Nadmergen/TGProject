package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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

const (
	maxBodyBytes           = 1 << 20 // 1 MiB
	redisTimeout           = 3 * time.Second
	dbTimeout              = 5 * time.Second
	registrationOTPTTL      = 5 * time.Minute
	twoFactorOTPTTL         = 5 * time.Minute
	initRegisterCooldownTTL = 60 * time.Second
	maxOTPAttempts          = 5
	otpAttemptWindow        = 10 * time.Minute
)

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return false
	}
	return true
}

func bearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return authHeader
}

func (as *AuthService) checkCooldown(ctx context.Context, key string, ttl time.Duration) error {
	ok, err := as.cache.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cooldown")
	}
	return nil
}

func (as *AuthService) registerAttempt(ctx context.Context, key string, max int64, window time.Duration) (blocked bool, err error) {
	n, err := as.cache.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err := as.cache.Expire(ctx, key, window).Err(); err != nil {
			return false, err
		}
	}
	return n > max, nil
}

// InitRegisterHandler – отправка кода на email
func (as *AuthService) InitRegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{Status: "error", Message: "Method not allowed"})
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = normalizeEmail(req.Email)
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return
	}

	// Генерация OTP
	otp := generateOTP()
	ctx, cancel := context.WithTimeout(r.Context(), redisTimeout)
	defer cancel()

	// Анти-спам: ограничиваем частоту отправки на email
	if err := as.checkCooldown(ctx, fmt.Sprintf("reg_otp_cooldown:%s", req.Email), initRegisterCooldownTTL); err != nil {
		if err.Error() == "cooldown" {
			respondJSON(w, http.StatusTooManyRequests, AuthResponse{Status: "error", Message: "Too many requests"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}

	// Сохраняем в Redis на 5 минут с префиксом reg_otp
	err := as.cache.Set(ctx, fmt.Sprintf("reg_otp:%s", req.Email), otp, registrationOTPTTL).Err()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to save OTP"})
		return
	}

	// Отправка email (мок)
	as.email.SendVerificationEmail(req.Email, otp, "Registration Code")

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Verification code sent to email",
	})
}

// VerifyCodeHandler – проверка кода и создание пользователя
func (as *AuthService) VerifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{Status: "error", Message: "Method not allowed"})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
		Phone    string `json:"phone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = normalizeEmail(req.Email)
	if req.Email == "" || req.Username == "" || req.Password == "" || req.Code == "" {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return
	}

	ctx, cancelRedis := context.WithTimeout(r.Context(), redisTimeout)
	defer cancelRedis()

	blocked, err := as.registerAttempt(ctx, fmt.Sprintf("reg_otp_attempts:%s", req.Email), maxOTPAttempts, otpAttemptWindow)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}
	if blocked {
		respondJSON(w, http.StatusTooManyRequests, AuthResponse{Status: "error", Message: "Too many attempts"})
		return
	}

	storedCode, err := as.cache.Get(ctx, fmt.Sprintf("reg_otp:%s", req.Email)).Result()
	if err != nil || storedCode != req.Code {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{
			Status:  "error",
			Message: "Invalid or expired verification code",
		})
		return
	}
	if err := as.cache.Del(ctx, fmt.Sprintf("reg_otp:%s", req.Email)).Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}
	_ = as.cache.Del(ctx, fmt.Sprintf("reg_otp_attempts:%s", req.Email)).Err()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}

	var userID int
	ctxDB, cancelDB := context.WithTimeout(r.Context(), dbTimeout)
	defer cancelDB()

	err = as.db.QueryRowContext(
		ctxDB,
		"INSERT INTO users (username, email, phone, password_hash, is_verified) VALUES ($1, $2, $3, $4, true) RETURNING id",
		req.Username, req.Email, req.Phone, string(hashedPassword),
	).Scan(&userID)
	if err != nil {
		log.Printf("❌ Registration error: %v", err)
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Registration failed"})
		return
	}

	token := generateToken()
	if _, err := as.db.ExecContext(
		ctxDB,
		"INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, token, time.Now().Add(30*24*time.Hour),
	); err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to create session"})
		return
	}

	log.Printf("✅ User registered: %s (ID: %d)", req.Username, userID)

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Registration successful",
		Token:   token,
		UserID:  userID,
	})
}

// LoginHandler – вход в систему
func (as *AuthService) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{Status: "error", Message: "Method not allowed"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return
	}

	var userID int
	var passwordHash string
	var has2FA bool
	ctxDB, cancelDB := context.WithTimeout(r.Context(), dbTimeout)
	defer cancelDB()

	err := as.db.QueryRowContext(
		ctxDB,
		"SELECT id, password_hash, has_2fa FROM users WHERE username = $1",
		req.Username,
	).Scan(&userID, &passwordHash, &has2FA)

	invalidCreds := func() {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{Status: "error", Message: "Invalid credentials"})
	}
	if err == sql.ErrNoRows {
		invalidCreds()
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		invalidCreds()
		return
	}

	if has2FA {
		code := generateOTP()
		ctxRedis, cancelRedis := context.WithTimeout(r.Context(), redisTimeout)
		defer cancelRedis()
		if err := as.cache.Set(ctxRedis, fmt.Sprintf("2fa_code:%d", userID), code, twoFactorOTPTTL).Err(); err != nil {
			respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
			return
		}
		var email string
		if err := as.db.QueryRowContext(ctxDB, "SELECT email FROM users WHERE id = $1", userID).Scan(&email); err != nil {
			respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
			return
		}
		as.email.SendVerificationEmail(email, code, "2FA Code")
		respondJSON(w, http.StatusOK, AuthResponse{
			Status:      "success",
			Message:     "2FA code sent to email",
			RequiresOTP: true,
			UserID:      userID,
		})
		return
	}

	token := generateToken()
	if _, err := as.db.ExecContext(
		ctxDB,
		"INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)",
		userID, token, time.Now().Add(30*24*time.Hour),
	); err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to create session"})
		return
	}

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "Login successful",
		Token:   token,
		UserID:  userID,
	})
}

// Verify2FAHandler – проверка 2FA
func (as *AuthService) Verify2FAHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{Status: "error", Message: "Method not allowed"})
		return
	}

	var req struct {
		UserID int    `json:"user_id"`
		Code   string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.UserID <= 0 || req.Code == "" {
		respondJSON(w, http.StatusBadRequest, AuthResponse{Status: "error", Message: "Invalid request"})
		return
	}

	ctx, cancelRedis := context.WithTimeout(r.Context(), redisTimeout)
	defer cancelRedis()

	blocked, err := as.registerAttempt(ctx, fmt.Sprintf("2fa_attempts:%d", req.UserID), maxOTPAttempts, otpAttemptWindow)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}
	if blocked {
		respondJSON(w, http.StatusTooManyRequests, AuthResponse{Status: "error", Message: "Too many attempts"})
		return
	}

	storedCode, err := as.cache.Get(ctx, fmt.Sprintf("2fa_code:%d", req.UserID)).Result()
	if err != nil || storedCode != req.Code {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{Status: "error", Message: "Invalid 2FA code"})
		return
	}
	if err := as.cache.Del(ctx, fmt.Sprintf("2fa_code:%d", req.UserID)).Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to process request"})
		return
	}
	_ = as.cache.Del(ctx, fmt.Sprintf("2fa_attempts:%d", req.UserID)).Err()

	token := generateToken()
	ctxDB, cancelDB := context.WithTimeout(r.Context(), dbTimeout)
	defer cancelDB()
	if _, err := as.db.ExecContext(
		ctxDB,
		"INSERT INTO sessions (user_id, token, expires_at) VALUES ($1, $2, $3)",
		req.UserID, token, time.Now().Add(30*24*time.Hour),
	); err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to create session"})
		return
	}

	respondJSON(w, http.StatusOK, AuthResponse{
		Status:  "success",
		Message: "2FA verified",
		Token:   token,
		UserID:  req.UserID,
	})
}

// LogoutHandler – выход
func (as *AuthService) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, AuthResponse{Status: "error", Message: "Method not allowed"})
		return
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		respondJSON(w, http.StatusUnauthorized, AuthResponse{Status: "error", Message: "Unauthorized"})
		return
	}
	ctxDB, cancelDB := context.WithTimeout(r.Context(), dbTimeout)
	defer cancelDB()
	if _, err := as.db.ExecContext(ctxDB, "DELETE FROM sessions WHERE token = $1", token); err != nil {
		respondJSON(w, http.StatusInternalServerError, AuthResponse{Status: "error", Message: "Failed to logout"})
		return
	}
	respondJSON(w, http.StatusOK, AuthResponse{Status: "success", Message: "Logged out"})
}