package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/redis/go-redis/v9"
)

type VoiceService struct {
	db    *sql.DB
	cache *redis.Client
}

func NewVoiceService(db *sql.DB, cache *redis.Client) *VoiceService {
	return &VoiceService{db: db, cache: cache}
}

// === ЗАГРУЗКА ГОЛОСОВОГО СООБЩЕНИЯ ===
func (vs *VoiceService) UploadVoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	userID := r.Header.Get("X-User-ID")
	recipientID := r.URL.Query().Get("recipient_id")

	if userID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "User ID required",
		})
		return
	}

	// Парсим multipart форму (максимум 50MB)
	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "File too large (max 50MB)",
		})
		return
	}

	file, handler, err := r.FormFile("voice")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "No voice file provided",
		})
		return
	}
	defer file.Close()

	// Создаём папку для голосовых сообщений
	voiceDir := "uploads/voice"
	os.MkdirAll(voiceDir, os.ModePerm)

	// Генерируем имя файла
	filename := fmt.Sprintf("%s_%d_%s", userID, time.Now().Unix(), handler.Filename)
	filePath := filepath.Join(voiceDir, filename)

	// Сохраняем файл на диск
	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ Failed to create file: %v\n", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to save file",
		})
		return
	}
	defer dst.Close()

	// Копируем файл
	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("❌ Failed to copy file: %v\n", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to save file",
		})
		return
	}

	// Сохраняем в БД
	var msgID int64
	err = vs.db.QueryRow(
		"INSERT INTO messages (sender_id, recipient_id, content, type, file_url, file_name, created_at) VALUES ($1, $2, $3, $4, $5, $6, NOW()) RETURNING id",
		userID, recipientID, "", "voice", filePath, handler.Filename,
	).Scan(&msgID)

	if err != nil {
		log.Printf("❌ Database error: %v\n", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to save message",
		})
		return
	}

	log.Printf("✅ Voice message uploaded: ID=%d, User=%s, Size=%d bytes\n", msgID, userID, handler.Size)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":       msgID,
		"status":   "uploaded",
		"file_url": filePath,
		"size":     handler.Size,
	})
}

// === СКАЧИВАНИЕ ГОЛОСОВОГО СООБЩЕНИЯ ===
func (vs *VoiceService) DownloadVoiceHandler(w http.ResponseWriter, r *http.Request) {
	fileURL := r.URL.Query().Get("file_url")

	if fileURL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "File URL required",
		})
		return
	}

	// Проверка безопасности
	if !isValidVoicePath(fileURL) {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "Invalid file path",
		})
		return
	}

	// Открываем файл
	file, err := os.Open(fileURL)
	if err != nil {
		log.Printf("❌ File not found: %s\n", fileURL)
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "File not found",
		})
		return
	}
	defer file.Close()

	// Получаем информацию о файле
	fileInfo, err := file.Stat()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to get file info",
		})
		return
	}

	// Устанавливаем заголовки
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"voice_%d.wav\"", time.Now().Unix()))

	// Отправляем файл
	if _, err := io.Copy(w, file); err != nil {
		log.Printf("❌ Failed to send file: %v\n", err)
		return
	}

	log.Printf("✅ Voice file downloaded: %s (%d bytes)\n", fileURL, fileInfo.Size())
}

// === УДАЛЕНИЕ ГОЛОСОВОГО СООБЩЕНИЯ ===
func (vs *VoiceService) DeleteVoiceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	var req struct {
		MessageID int64 `json:"message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request",
		})
		return
	}

	userID := r.Header.Get("X-User-ID")

	// Получаем путь к файлу
	var fileURL string
	err := vs.db.QueryRow(
		"SELECT file_url FROM messages WHERE id = $1 AND sender_id = $2 AND type = 'voice'",
		req.MessageID, userID,
	).Scan(&fileURL)

	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Message not found",
		})
		return
	}

	// Удаляем файл с диска
	if fileURL != "" && isValidVoicePath(fileURL) {
		if err := os.Remove(fileURL); err != nil {
			log.Printf("⚠️ Failed to delete file: %v\n", err)
		}
	}

	// Удаляем из БД
	_, err = vs.db.Exec(
		"DELETE FROM messages WHERE id = $1 AND sender_id = $2",
		req.MessageID, userID,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to delete message",
		})
		return
	}

	log.Printf("✅ Voice message deleted: ID=%d\n", req.MessageID)

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
	})
}
