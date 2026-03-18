package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log" // добавлен
	"net/http"
	"strconv"
	"strings"
	"time"

	"context"
	"github.com/redis/go-redis/v9"
	"github.com/minio/minio-go/v7"
)

type MessageService struct {
	db    *sql.DB
	cache *redis.Client
	hub   *Hub
}

type MessageResponse struct {
	ID          int64      `json:"id"`
	SenderID    int64      `json:"sender_id"`
	RecipientID int64      `json:"recipient_id"`
	Content     string     `json:"content"`
	Type        string     `json:"type"` // text, image, file, voice
	FileURL     string     `json:"file_url"`
	FileName    string     `json:"file_name"`
	DeliveredAt *time.Time `json:"delivered_at"`
	ReadAt      *time.Time `json:"read_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func NewMessageService(db *sql.DB, cache *redis.Client, hub *Hub) *MessageService {
	return &MessageService{db: db, cache: cache, hub: hub}
}

// SendMessageHandler – отправка сообщения (текст, файл, голос)
func (ms *MessageService) SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
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

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var msgID int64
	err := ms.db.QueryRow(
		"INSERT INTO messages (sender_id, recipient_id, content, type, file_url, file_name, created_at) VALUES ($1, $2, $3, $4, $5, $6, NOW()) RETURNING id",
		userID, req.RecipientID, req.Content, req.Type, req.FileURL, req.FileName,
	).Scan(&msgID)
	if err != nil {
		log.Printf("❌ SendMessage error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to send message"})
		return
	}

	// Инвалидируем кеш
	ms.cache.Del(context.Background(), fmt.Sprintf("messages:%d", userID))
	ms.cache.Del(context.Background(), fmt.Sprintf("messages:%d", req.RecipientID))

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":     msgID,
		"status": "sent",
	})

	if ms.hub != nil {
		ms.hub.SendToUser(int64(req.RecipientID), map[string]interface{}{
			"event":        "message_created",
			"id":           msgID,
			"sender_id":    userID,
			"recipient_id": req.RecipientID,
			"content":      req.Content,
			"type":         req.Type,
			"file_url":     req.FileURL,
			"file_name":    req.FileName,
			"created_at":   time.Now(),
		})
	}
}

// GetMessagesHandler – получение истории с пагинацией
func (ms *MessageService) GetMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	page := r.URL.Query().Get("page")
	if page == "" {
		page = "1"
	}
	pageNum, _ := strconv.Atoi(page)
	offset := (pageNum - 1) * 50

	cacheKey := fmt.Sprintf("messages:%d:page:%s", userID, page)
	if cached, err := ms.cache.Get(context.Background(), cacheKey).Result(); err == nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, cached)
		return
	}

	rows, err := ms.db.Query(`
		SELECT id, sender_id, recipient_id, content, type, file_url, file_name, delivered_at, read_at, created_at
		FROM messages
		WHERE sender_id = $1 OR recipient_id = $1
		ORDER BY created_at DESC
		LIMIT 50 OFFSET $2
	`, userID, offset)
	if err != nil {
		log.Printf("❌ GetMessages error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	var messages []MessageResponse
	var deliveredIDs []int64
	var deliveredSenders []int64
	for rows.Next() {
		var m MessageResponse
		rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Content, &m.Type, &m.FileURL, &m.FileName, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt)
		messages = append(messages, m)

		if m.RecipientID == userID && m.DeliveredAt == nil {
			deliveredIDs = append(deliveredIDs, m.ID)
			deliveredSenders = append(deliveredSenders, m.SenderID)
		}
	}

	data, _ := json.Marshal(messages)
	ms.cache.Set(context.Background(), cacheKey, string(data), 5*time.Minute)

	// Mark as delivered (best-effort) and notify senders.
	if len(deliveredIDs) > 0 {
		for i, id := range deliveredIDs {
			_, _ = ms.db.Exec("UPDATE messages SET delivered_at = NOW() WHERE id = $1 AND recipient_id = $2 AND delivered_at IS NULL", id, userID)
			if ms.hub != nil {
				ms.hub.SendToUser(deliveredSenders[i], map[string]interface{}{
					"event":        "message_delivered",
					"id":           id,
					"delivered_at": time.Now(),
				})
			}
		}
	}

	respondJSON(w, http.StatusOK, messages)
}

// SearchMessagesHandler – поиск сообщений по содержимому
func (ms *MessageService) SearchMessagesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Query required"})
		return
	}

	rows, err := ms.db.Query(`
		SELECT id, sender_id, recipient_id, content, type, file_url, file_name, delivered_at, read_at, created_at
		FROM messages
		WHERE (sender_id = $1 OR recipient_id = $1)
		AND content ILIKE $2
		ORDER BY created_at DESC
		LIMIT 100
	`, userID, "%"+query+"%")
	if err != nil {
		log.Printf("❌ SearchMessages error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	var messages []MessageResponse
	for rows.Next() {
		var m MessageResponse
		rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Content, &m.Type, &m.FileURL, &m.FileName, &m.DeliveredAt, &m.ReadAt, &m.CreatedAt)
		messages = append(messages, m)
	}

	respondJSON(w, http.StatusOK, messages)
}

// DeleteMessageHandler – удаление сообщения (только для отправителя)
func (ms *MessageService) DeleteMessageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var fileURL sql.NullString
	var msgType sql.NullString
	err := ms.db.QueryRow("SELECT file_url, type FROM messages WHERE id = $1 AND sender_id = $2", req.ID, userID).Scan(&fileURL, &msgType)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Message not found or not yours"})
		return
	}
	if err != nil {
		log.Printf("❌ DeleteMessage lookup error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}

	result, err := ms.db.Exec("DELETE FROM messages WHERE id = $1 AND sender_id = $2", req.ID, userID)
	if err != nil {
		log.Printf("❌ DeleteMessage error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Message not found or not yours"})
		return
	}

	// Best-effort cleanup of attachment (S3 or local legacy path).
	if fileURL.Valid && strings.TrimSpace(fileURL.String) != "" {
		key := strings.TrimSpace(fileURL.String)
		if strings.HasPrefix(key, "uploads/") {
			if s3, bucket, _, err := newS3ClientFromEnv(); err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s3.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
			}
		}
	}

	ms.cache.Del(context.Background(), fmt.Sprintf("messages:%d", userID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// MarkAsReadHandler – пометить сообщение как прочитанное
func (ms *MessageService) MarkAsReadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	_, err := ms.db.Exec("UPDATE messages SET read_at = NOW() WHERE id = $1 AND recipient_id = $2", req.ID, userID)
	if err != nil {
		log.Printf("❌ MarkAsRead error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}

	if ms.hub != nil {
		var senderID int64
		var readAt time.Time
		err := ms.db.QueryRow("SELECT sender_id, read_at FROM messages WHERE id = $1", req.ID).Scan(&senderID, &readAt)
		if err == nil && senderID > 0 {
			ms.hub.SendToUser(senderID, map[string]interface{}{
				"event":   "message_read",
				"id":      req.ID,
				"read_at": readAt,
			})
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "marked as read"})
}