package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type ProfileService struct {
	db *sql.DB
}

func NewProfileService(db *sql.DB) *ProfileService {
	return &ProfileService{db: db}
}

func (ps *ProfileService) UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Status   string `json:"status"`
		Phone    string `json:"phone"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Status = strings.TrimSpace(req.Status)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "username required"})
		return
	}

	var username string
	var status sql.NullString
	err := ps.db.QueryRow(
		`UPDATE users
		 SET username = $1,
		     status = NULLIF($2, ''),
		     phone = NULLIF($3, ''),
		     email = COALESCE(NULLIF($4, ''), email),
		     updated_at = NOW()
		 WHERE id = $5
		 RETURNING username, status`,
		req.Username, req.Status, req.Phone, req.Email, userID,
	).Scan(&username, &status)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to update profile"})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"username": username,
		"status":   status.String,
	})
}

