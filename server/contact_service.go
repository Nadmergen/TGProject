package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"context"
	"github.com/redis/go-redis/v9"
)

type ContactService struct {
	db    *sql.DB
	cache *redis.Client // ← ДОБАВИЛ
}

type Contact struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Username string `json:"username"`
}

type SyncContactsRequest struct {
	Contacts []Contact `json:"contacts"`
}

func NewContactService(db *sql.DB, cache *redis.Client) *ContactService { // ← ИЗМЕНЕНО (добавил cache)
	return &ContactService{db: db, cache: cache}
}

func (cs *ContactService) GetContactsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")

	// Проверяем кеш
	cacheKey := fmt.Sprintf("contacts:%s", userID)
	cachedContacts, _ := cs.cache.Get(context.Background(), cacheKey).Result()
	if cachedContacts != "" {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, cachedContacts)
		return
	}

	rows, err := cs.db.Query(`
		SELECT c.id, c.name, c.phone, u.username
		FROM contacts c
		LEFT JOIN users u ON c.contact_id = u.id
		WHERE c.user_id = $1
		ORDER BY c.name ASC
	`, userID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Username)
		contacts = append(contacts, c)
	}

	// Кешируем на 1 час
	data, _ := json.Marshal(contacts)
	cs.cache.Set(context.Background(), cacheKey, string(data), 1*time.Hour)

	respondJSON(w, http.StatusOK, contacts)
}

func (cs *ContactService) AddContactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)

	cs.db.Exec(
		"INSERT INTO contacts (user_id, name, phone) VALUES ($1, $2, $3)",
		userID, req["name"], req["phone"],
	)

	// Инвалидируем кеш
	cs.cache.Del(context.Background(), fmt.Sprintf("contacts:%s", userID))

	respondJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (cs *ContactService) SyncContactsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	var req SyncContactsRequest
	json.NewDecoder(r.Body).Decode(&req)

	for _, contact := range req.Contacts {
		cs.db.Exec(
			"INSERT INTO contacts (user_id, name, phone) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
			userID, contact.Name, contact.Phone,
		)
	}

	// Инвалидируем кеш
	cs.cache.Del(context.Background(), fmt.Sprintf("contacts:%s", userID))

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "synced",
		"count":  len(req.Contacts),
	})
}
