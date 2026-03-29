package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var (
	db           *sql.DB
	cache        *redis.Client
	emailService *SMTPEmailService
)

func init() {
	godotenv.Load()
	// defaults for local development (docker-compose)
	if os.Getenv("DB_HOST") == "" {
		os.Setenv("DB_HOST", "localhost")
	}
	if os.Getenv("DB_PORT") == "" {
		os.Setenv("DB_PORT", "5432")
	}
	if os.Getenv("DB_USER") == "" {
		os.Setenv("DB_USER", "messenger")
	}
	if os.Getenv("DB_PASSWORD") == "" {
		os.Setenv("DB_PASSWORD", "your_secure_password_123")
	}
	if os.Getenv("DB_NAME") == "" {
		os.Setenv("DB_NAME", "messenger_db")
	}
	if os.Getenv("REDIS_URL") == "" {
		os.Setenv("REDIS_URL", "localhost:6379")
	}
	// Инициализируем email сервис
	emailService = NewSMTPEmailService()
}

func main() {
	// === ПРОВЕРКА КОНФИГУРАЦИИ ===
	if err := checkConfiguration(); err != nil {
		log.Printf("⚠️ Configuration warning: %v\n", err)
	}

	// Инициализация PostgreSQL
	db = initPostgres()
	defer db.Close()

	// Инициализация Redis
	cache = initRedis()
	defer cache.Close()

	// Connection pool оптимизация
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Инициализация Hub
	hub := NewHub()
	go hub.Run()

	// Middleware
	mw := NewMiddleware(db)

	// Service layer
	authService := NewAuthService(db, cache)
	messageService := NewMessageService(db, cache, hub)
	contactService := NewContactService(db, cache)
	voiceService := NewVoiceService(db, cache)
	profileService := NewProfileService(db)
	uploadService, uploadErr := NewUploadService(db)
	if uploadErr != nil {
		log.Printf("⚠️ Upload service disabled: %v\n", uploadErr)
	}

	// API routes
	mux := http.NewServeMux()

	// === CORS Middleware ===
	wrappedMux := enableCORS(mux)

	// ===== AUTH ENDPOINTS =====
	mux.HandleFunc("/api/auth/init-register", authService.InitRegisterHandler)
	mux.HandleFunc("/api/auth/verify-code", authService.VerifyCodeHandler)
	mux.HandleFunc("/api/auth/login", authService.LoginHandler)
	mux.HandleFunc("/api/auth/verify-2fa", authService.Verify2FAHandler)
	mux.HandleFunc("/api/auth/forgot-password", authService.ForgotPasswordInitHandler)
	mux.HandleFunc("/api/auth/reset-password", authService.ForgotPasswordVerifyHandler)
	mux.Handle("/api/auth/qr-init", mw.AuthRequired(http.HandlerFunc(authService.QRAuthInitHandler)))
	mux.HandleFunc("/api/auth/qr-login", authService.QRAuthLoginHandler)
	mux.HandleFunc("/api/auth/logout", authService.LogoutHandler)
	mux.Handle("/api/messages/read", mw.AuthRequired(http.HandlerFunc(messageService.MarkAsReadHandler)))

	// ===== PROFILE =====
	mux.Handle("/api/profile/update", mw.AuthRequired(http.HandlerFunc(profileService.UpdateProfileHandler)))

	// ===== WEBSOCKET =====
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, db, w, r)
	})

	// ===== MESSAGES ENDPOINTS =====
	mux.Handle("/api/messages", mw.AuthRequired(http.HandlerFunc(messageService.GetMessagesHandler)))
	mux.Handle("/api/messages/send", mw.AuthRequired(http.HandlerFunc(messageService.SendMessageHandler)))
	mux.Handle("/api/messages/delete", mw.AuthRequired(http.HandlerFunc(messageService.DeleteMessageHandler)))
	mux.Handle("/api/messages/search", mw.AuthRequired(http.HandlerFunc(messageService.SearchMessagesHandler)))

	// ===== CONTACTS ENDPOINTS =====
	mux.Handle("/api/contacts", mw.AuthRequired(http.HandlerFunc(contactService.GetContactsHandler)))
	mux.Handle("/api/contacts/add", mw.AuthRequired(http.HandlerFunc(contactService.AddContactHandler)))
	mux.Handle("/api/contacts/sync", mw.AuthRequired(http.HandlerFunc(contactService.SyncContactsHandler)))

	// ===== VOICE MESSAGES ENDPOINTS =====
	mux.Handle("/api/voice/upload", mw.AuthRequired(http.HandlerFunc(voiceService.UploadVoiceHandler)))
	mux.Handle("/api/voice/download", mw.AuthRequired(http.HandlerFunc(voiceService.DownloadVoiceHandler)))

	// ===== UPLOADS (S3/MinIO) =====
	if uploadService != nil {
		mux.Handle("/api/uploads/init", mw.AuthRequired(http.HandlerFunc(uploadService.InitUploadHandler)))
		mux.Handle("/api/uploads/complete", mw.AuthRequired(http.HandlerFunc(uploadService.CompleteUploadHandler)))
		mux.Handle("/api/uploads/download", mw.AuthRequired(http.HandlerFunc(uploadService.PresignDownloadHandler)))
	}

	// ===== CALLS ENDPOINTS =====
	mux.HandleFunc("/api/calls/start", handleCallStart)
	mux.HandleFunc("/api/calls/end", handleCallEnd)

	// ===== HEALTH CHECK =====
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// ===== SERVER =====
	server := &http.Server{
		Addr:         ":" + os.Getenv("PORT"),
		Handler:      wrappedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("🛑 Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Println("❌ Shutdown error:", err)
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Сервер запущен на http://localhost:%s\n", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// === ПРОВЕРКА КОНФИГУРАЦИИ ===
func checkConfiguration() error {
	host := os.Getenv("SMTP_HOST")
	from := os.Getenv("SMTP_FROM")
	pass := os.Getenv("SMTP_PASSWORD")

	if host == "" || from == "" || pass == "" {
		log.Println("⚠️ SMTP not fully configured")
		return fmt.Errorf("SMTP credentials incomplete")
	}

	// Тестируем только SMTP
	emailSvc := NewSMTPEmailService()
	if err := emailSvc.TestConnection(); err != nil {
		log.Printf("⚠️ SMTP connection failed: %v\n", err)
		return err
	}

	return nil
}

// === CORS Middleware ===
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// === POSTGRES ===
func initPostgres() *sql.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("❌ Database connection error:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("❌ Database ping error:", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		phone TEXT UNIQUE,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		status TEXT,
		avatar_url TEXT,
		has_2fa BOOLEAN DEFAULT false,
		is_verified BOOLEAN DEFAULT false,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id BIGSERIAL PRIMARY KEY,
		sender_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		recipient_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
		content TEXT,
		type TEXT DEFAULT 'text',
		file_url TEXT,
		file_name TEXT,
		read_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (sender_id) REFERENCES users(id),
		FOREIGN KEY (recipient_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS contacts (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		contact_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name TEXT,
		phone TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, contact_id)
	);

	CREATE TABLE IF NOT EXISTS calls (
		id BIGSERIAL PRIMARY KEY,
		caller_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		recipient_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		type TEXT DEFAULT 'voice',
		duration INTEGER,
		status TEXT DEFAULT 'missed',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (caller_id) REFERENCES users(id),
		FOREIGN KEY (recipient_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token TEXT UNIQUE NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_id);
	CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient_id);
	CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
	CREATE INDEX IF NOT EXISTS idx_contacts_user ON contacts(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`

	if _, err := db.Exec(schema); err != nil {
		log.Println("⚠️ Schema creation warning:", err)
	}

	// Lightweight migrations (best-effort).
	if _, err := db.Exec(`ALTER TABLE messages ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMP;`); err != nil {
		log.Println("⚠️ Migration warning (delivered_at):", err)
	}

	log.Println("✅ PostgreSQL connected")
	return db
}

// === REDIS ===
func initRedis() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_URL"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Println("⚠️ Redis connection failed:", err)
	} else {
		log.Println("✅ Redis connected")
	}

	return client
}

// === WEBSOCKET ===
func handleWebSocket(hub *Hub, db *sql.DB, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	// Authenticate via query param token (browser-friendly).
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var userID int64
	var username string
	err := db.QueryRow(
		`SELECT u.id, u.username
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token = $1 AND s.expires_at > NOW()`,
		token,
	).Scan(&userID, &username)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("❌ WebSocket upgrade error:", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan interface{}, 512),
		userID: userID,
		username: username,
	}

	hub.register <- client

	go client.readPump()
	go client.writePump()
}

func handleCallStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"call_started"}`)
}

func handleCallEnd(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"call_ended"}`)
}
