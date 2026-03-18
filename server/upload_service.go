package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type UploadService struct {
	db     *sql.DB
	s3     *minio.Client
	bucket string
}

func NewUploadService(db *sql.DB) (*UploadService, error) {
	client, bucket, region, err := newS3ClientFromEnv()
	if err != nil {
		return nil, err
	}

	// Ensure bucket exists (best-effort).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, bucket)
	if err == nil && !exists {
		_ = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region})
	}

	return &UploadService{db: db, s3: client, bucket: bucket}, nil
}

type initUploadRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type initUploadResponse struct {
	ObjectKey string `json:"object_key"`
	UploadURL string `json:"upload_url"`
}

// InitUploadHandler returns a presigned PUT URL.
func (us *UploadService) InitUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req initUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}
	req.FileName = strings.TrimSpace(req.FileName)
	req.ContentType = strings.TrimSpace(req.ContentType)
	if req.FileName == "" || len(req.FileName) > 255 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid file_name"})
		return
	}
	if req.Size <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid size"})
		return
	}

	safeName := sanitizeFileName(req.FileName)
	objectKey := fmt.Sprintf("uploads/%d/%s/%s", userID, uuid.NewString(), safeName)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	u, err := us.s3.PresignedPutObject(ctx, us.bucket, objectKey, 15*time.Minute)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to init upload"})
		return
	}

	respondJSON(w, http.StatusOK, initUploadResponse{
		ObjectKey: objectKey,
		UploadURL: u.String(),
	})
}

type completeUploadRequest struct {
	RecipientID  int64  `json:"recipient_id"`
	Type         string `json:"type"` // file|image|video|voice
	Content      string `json:"content,omitempty"`
	ObjectKey    string `json:"object_key"`
	FileName     string `json:"file_name"`
	ContentType  string `json:"content_type,omitempty"`
	Size         int64  `json:"size,omitempty"`
	DurationMS   int64  `json:"duration_ms,omitempty"`
	ThumbKey     string `json:"thumb_key,omitempty"`
	ThumbMime    string `json:"thumb_mime,omitempty"`
	ThumbSize    int64  `json:"thumb_size,omitempty"`
	OriginalName string `json:"original_name,omitempty"`
}

type completeUploadResponse struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	FileURL     string `json:"file_url"`
	DownloadURL string `json:"download_url"`
}

func (us *UploadService) CompleteUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req completeUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.ObjectKey = strings.TrimSpace(req.ObjectKey)
	req.FileName = strings.TrimSpace(req.FileName)
	if req.RecipientID <= 0 || req.ObjectKey == "" || req.FileName == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}
	switch req.Type {
	case "file", "image", "video", "voice":
	default:
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid type"})
		return
	}

	// Basic authorization: caller must be sender, recipient exists.
	var recipientExists bool
	if err := us.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", req.RecipientID).Scan(&recipientExists); err != nil || !recipientExists {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid recipient"})
		return
	}

	// Ensure object exists in bucket (best-effort head).
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := us.s3.StatObject(ctx, us.bucket, req.ObjectKey, minio.StatObjectOptions{}); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Upload not found"})
		return
	}

	var msgID int64
	err := us.db.QueryRow(
		"INSERT INTO messages (sender_id, recipient_id, content, type, file_url, file_name, created_at) VALUES ($1, $2, $3, $4, $5, $6, NOW()) RETURNING id",
		userID, req.RecipientID, req.Content, req.Type, req.ObjectKey, req.FileName,
	).Scan(&msgID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save message"})
		return
	}

	dl, err := us.s3.PresignedGetObject(ctx, us.bucket, req.ObjectKey, 30*time.Minute, nil)
	if err != nil {
		respondJSON(w, http.StatusOK, completeUploadResponse{ID: msgID, Status: "sent", FileURL: req.ObjectKey, DownloadURL: ""})
		return
	}

	respondJSON(w, http.StatusOK, completeUploadResponse{
		ID:          msgID,
		Status:      "sent",
		FileURL:     req.ObjectKey,
		DownloadURL: dl.String(),
	})
}

// PresignDownloadHandler returns a presigned GET URL for stored object_key.
func (us *UploadService) PresignDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	userID, ok := getUserID(r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	objectKey := strings.TrimSpace(r.URL.Query().Get("object_key"))
	if objectKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "object_key required"})
		return
	}

	// Access control: requester must be sender or recipient of message referencing this object.
	var allowed bool
	err := us.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM messages
			WHERE file_url = $1 AND (sender_id = $2 OR recipient_id = $2)
		)`,
		objectKey, userID,
	).Scan(&allowed)
	if err != nil || !allowed {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	dl, err := us.s3.PresignedGetObject(ctx, us.bucket, objectKey, 30*time.Minute, nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to presign"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"download_url": dl.String()})
}

func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "file"
	}
	return name
}

func newS3ClientFromEnv() (*minio.Client, string, string, error) {
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
	region := strings.TrimSpace(os.Getenv("S3_REGION"))
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, "", "", fmt.Errorf("missing S3 config: S3_ENDPOINT/S3_ACCESS_KEY/S3_SECRET_KEY/S3_BUCKET")
	}

	useSSL := true
	if v := strings.TrimSpace(os.Getenv("S3_USE_SSL")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err == nil {
			useSSL = parsed
		}
	}

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	}
	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, "", "", err
	}
	return client, bucket, region, nil
}

