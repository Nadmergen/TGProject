package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
)

// === ГЕНЕРАЦИЯ OTP КОДА (6 ЦИФР) ===
func generateOTP() string {
	b := make([]byte, 3)
	rand.Read(b)
	num := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
	return fmt.Sprintf("%06d", num)
}

// === ГЕНЕРАЦИЯ ТОКЕНА (32 БАЙТА) ===
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// === ОТВЕТ JSON ===
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// === ПРОВЕРКА ВАЛИДНОГО ПУТИ (БЕЗОПАСНОСТЬ) ===
func isValidVoicePath(fileURL string) bool {
	// Убедимся что файл в папке uploads/voice
	return filepath.HasPrefix(fileURL, "uploads/voice/")
}

// === ПРОВЕРКА ВАЛИДНОГО ФАЙЛА ===
func isValidPath(fileURL string) bool {
	// Базовая проверка безопасности
	return !filepath.IsAbs(fileURL)
}
