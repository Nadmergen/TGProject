package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"context"
	"github.com/redis/go-redis/v9"
)

type MessageService struct {
	db    *sql.DB
	cache *redis.Client
}

type MessageResponse struct {
	ID        int64      `json:"id"`
	SenderID  int64      `json:"sender_id"`
	Content   string     `json:"content"`
	Type      string     `json:"type"`
	FileURL   string     `json:"file_url"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func NewMessageService(db *sql.DB, cache *redis.Client) *MessageService {
	return &MessageService{db: db, cache: cache}
}

func (ms *MessageService) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RecipientID int    `json:"recipient_id"`
		Content     string `json:"content"`
		Type        string `json:"type"`
		FileURL     string `json:"file_url"`
		FileName    string `json:"file_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	userID := r.Header.Get("X-User-ID")

	result, err := ms.db.Exec(
		"INSERT INTO messages (sender_id, recipient_id, content, type, file_url, file_name) VALUES ($1, $2, $3, $4, $5, $6)",
		userID, req.RecipientID, req.Content, req.Type, req.FileURL, req.FileName,
	)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to send message"})
		return
	}

	msgID, _ := result.LastInsertId()

	// Инвалидируем кеш
	ms.cache.Del(context.Background(), fmt.Sprintf("messages:%s", userID))
	ms.cache.Del(context.Background(), fmt.Sprintf("messages:%s", req.RecipientID))

	respondJSON(w, http.StatusOK, map[string]interface{}{"id": msgID, "status": "sent"})
}

func (ms *MessageService) GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}

	pageNum, _ := strconv.Atoi(page)
	offset := (pageNum - 1) * 50

	// Проверяем кеш
	cacheKey := fmt.Sprintf("messages:%s:page:%s", userID, page)
	cachedMessages, _ := ms.cache.Get(context.Background(), cacheKey).Result()
	if cachedMessages != "" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, cachedMessages)
		return
	}

	rows, err := ms.db.Query(`
		SELECT id, sender_id, content, type, file_url, read_at, created_at 
		FROM messages 
		WHERE sender_id = $1 OR recipient_id = $1
		ORDER BY created_at DESC
		LIMIT 50 OFFSET $2
	`, userID, offset)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	var messages []MessageResponse
	for rows.Next() {
		var msg MessageResponse
		rows.Scan(&msg.ID, &msg.SenderID, &msg.Content, &msg.Type, &msg.FileURL, &msg.ReadAt, &msg.CreatedAt)
		messages = append(messages, msg)
	}

	// Кешируем на 5 минут
	data, _ := json.Marshal(messages)
	ms.cache.Set(context.Background(), cacheKey, string(data), 5*time.Minute)

	respondJSON(w, http.StatusOK, messages)
}

func (ms *MessageService) SearchMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	query := r.URL.Query().Get("q")

	if query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Query required"})
		return
	}

	rows, err := ms.db.Query(`
		SELECT id, sender_id, content, type, file_url, read_at, created_at 
		FROM messages 
		WHERE (sender_id = $1 OR recipient_id = $1) 
		AND content ILIKE $2
		ORDER BY created_at DESC
		LIMIT 100
	`, userID, "%"+query+"%")

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	var messages []MessageResponse
	for rows.Next() {
		var msg MessageResponse
		rows.Scan(&msg.ID, &msg.SenderID, &msg.Content, &msg.Type, &msg.FileURL, &msg.ReadAt, &msg.CreatedAt)
		messages = append(messages, msg)
	}

	respondJSON(w, http.StatusOK, messages)
}

func (ms *MessageService) DeleteMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req map[string]int64
	json.NewDecoder(r.Body).Decode(&req)
	msgID := req["id"]
	userID := r.Header.Get("X-User-ID")

	ms.db.Exec("DELETE FROM messages WHERE id = $1 AND sender_id = $2", msgID, userID)

	// Инвалидируем кеш
	ms.cache.Del(context.Background(), fmt.Sprintf("messages:%s", userID))

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
