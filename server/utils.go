package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
)

// generateOTP создает 6-значный код
func generateOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(900000))
	return fmt.Sprintf("%06d", n.Uint64()+100000)
}

// generateToken создает криптографически стойкий токен сессии
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// respondJSON отправляет JSON ответ
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// getUserID извлекает userID из контекста запроса
func getUserID(r *http.Request) (int64, bool) {
	val := r.Context().Value("userID")
	if val == nil {
		return 0, false
	}
	id, ok := val.(int64)
	return id, ok
}