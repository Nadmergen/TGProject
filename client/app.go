package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	_ "modernc.org/sqlite"
)

// Message соответствует структуре сервера
type Message struct {
	ID          int64     `json:"id"`
	SenderID    int       `json:"sender_id"`
	RecipientID int       `json:"recipient_id"`
	Content     string    `json:"content"`
	Type        string    `json:"type"`
	FileURL     string    `json:"file_url"`
	FileName    string    `json:"file_name"`
	CreatedAt   time.Time `json:"created_at"`
	// Для удобства добавляем имя отправителя (заполняется отдельно)
	SenderName string `json:"sender_name,omitempty"`
}

// AuthResponse для парсинга ответов сервера
type AuthResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	Token       string `json:"token,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
	RequiresOTP bool   `json:"requires_otp,omitempty"`
}

type App struct {
	ctx         context.Context
	db          *sql.DB // локальная SQLite (офлайн-режим)
	offlineMode bool    // true если работаем без сервера
	httpClient  *http.Client
	baseURL     string // адрес сервера (из конфига)
	token       string // токен авторизации
	userID      int    // ID текущего пользователя
	username    string // username
	recipientID int    // ID собеседника (активный чат)
	wsConn      *websocket.Conn
	wsMutex     sync.Mutex
	done        chan struct{}
}

func NewApp() *App {
	return &App{
		offlineMode: false, // по умолчанию онлайн
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		baseURL:     "http://localhost:8080", // замените на реальный адрес сервера
		done:        make(chan struct{}),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Инициализация локальной БД (на случай офлайн-режима)
	db, err := sql.Open("sqlite", "./chat.db")
	if err != nil {
		fmt.Println("Ошибка открытия локальной БД:", err)
	} else {
		a.db = db
		// Создаём таблицы, если их нет
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT, 
			username TEXT UNIQUE, 
			password TEXT
		)`)
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT, 
			sender TEXT, 
			content TEXT, 
			type TEXT, 
			file_url TEXT, 
			file_name TEXT
		)`)
	}
}

// ========== АВТОРИЗАЦИЯ (Онлайн) ==========

// InitRegister отправляет email для получения кода подтверждения
func (a *App) InitRegister(email string) (map[string]interface{}, error) {
	if a.offlineMode {
		return nil, fmt.Errorf("offline mode: registration not available")
	}
	reqBody := map[string]string{"email": email}
	data, _ := json.Marshal(reqBody)
	resp, err := a.httpClient.Post(a.baseURL+"/api/auth/init-register", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(result.Message)
	}
	return map[string]interface{}{
		"status":  result.Status,
		"message": result.Message,
	}, nil
}

// VerifyCode завершает регистрацию с кодом
func (a *App) VerifyCode(email, code, username, password, phone string) (map[string]interface{}, error) {
	if a.offlineMode {
		return nil, fmt.Errorf("offline mode: registration not available")
	}
	reqBody := map[string]string{
		"email":    email,
		"code":     code,
		"username": username,
		"password": password,
		"phone":    phone,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := a.httpClient.Post(a.baseURL+"/api/auth/verify-code", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(result.Message)
	}

	// Сохраняем данные
	a.token = result.Token
	a.userID = result.UserID
	a.username = username
	a.offlineMode = false

	// Подключаем WebSocket
	go a.connectWebSocket()

	return map[string]interface{}{
		"status":  result.Status,
		"message": result.Message,
		"token":   result.Token,
		"user_id": result.UserID,
	}, nil
}

// Login выполняет вход
func (a *App) Login(username, password string) (map[string]interface{}, error) {
	if a.offlineMode {
		// Офлайн-режим: проверяем локально
		var storedPass string
		err := a.db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&storedPass)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("пользователь не найден")
		}
		if storedPass != password {
			return nil, fmt.Errorf("неверный пароль")
		}
		a.username = username
		a.offlineMode = true
		return map[string]interface{}{
			"status":  "success",
			"message": "Logged in offline",
		}, nil
	}

	// Онлайн-режим
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := a.httpClient.Post(a.baseURL+"/api/auth/login", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(result.Message)
	}

	// Если требуется 2FA
	if result.RequiresOTP {
		return map[string]interface{}{
			"requires_otp": true,
			"user_id":      result.UserID,
			"message":      result.Message,
		}, nil
	}

	// Успешный вход
	a.token = result.Token
	a.userID = result.UserID
	a.username = username
	a.offlineMode = false

	go a.connectWebSocket()

	return map[string]interface{}{
		"status":  result.Status,
		"message": result.Message,
		"token":   result.Token,
		"user_id": result.UserID,
	}, nil
}

// Verify2FA подтверждает двухфакторный код
func (a *App) Verify2FA(userID int, code string) (map[string]interface{}, error) {
	if a.offlineMode {
		return nil, fmt.Errorf("offline mode: 2FA not supported")
	}
	reqBody := map[string]interface{}{
		"user_id": userID,
		"code":    code,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := a.httpClient.Post(a.baseURL+"/api/auth/verify-2fa", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(result.Message)
	}

	a.token = result.Token
	a.userID = result.UserID
	// username не возвращается, можно получить отдельно
	a.offlineMode = false

	go a.connectWebSocket()

	return map[string]interface{}{
		"status":  result.Status,
		"message": result.Message,
		"token":   result.Token,
		"user_id": result.UserID,
	}, nil
}

// SetRecipient устанавливает ID собеседника (должен вызываться перед отправкой сообщения)
func (a *App) SetRecipient(recipientID int) {
	a.recipientID = recipientID
}

// ========== РАБОТА С ЧАТОМ ==========

// SendMessage отправляет сообщение (сигнатура сохранена для совместимости с фронтендом)
func (a *App) SendMessage(sender, content, msgType, fileUrl, fileName string) error {
	if a.offlineMode {
		// Офлайн: сохраняем локально
		res, err := a.db.Exec("INSERT INTO messages (sender, content, type, file_url, file_name) VALUES (?, ?, ?, ?, ?)",
			sender, content, msgType, fileUrl, fileName)
		if err != nil {
			return err
		}
		lastId, _ := res.LastInsertId()
		// Эмитим событие для фронтенда
		runtime.EventsEmit(a.ctx, "new_msg", Message{
			ID:       lastId,
			SenderID: 0, // неизвестно
			Content:  content,
			Type:     msgType,
			FileURL:  fileUrl,
			FileName: fileName,
		})
		return nil
	}

	// Онлайн: отправляем на сервер
	if a.token == "" {
		return fmt.Errorf("not authenticated")
	}
	if a.recipientID == 0 {
		return fmt.Errorf("recipient not set, call SetRecipient first")
	}

	reqBody := map[string]interface{}{
		"recipient_id": a.recipientID,
		"content":      content,
		"type":         msgType,
		"file_url":     fileUrl,
		"file_name":    fileName,
	}
	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", a.baseURL+"/api/messages/send", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}
	return nil
}

// GetHistory получает историю сообщений (онлайн) или из локальной БД (офлайн)
func (a *App) GetHistory() ([]Message, error) {
	if a.offlineMode {
		rows, err := a.db.Query("SELECT id, sender, content, type, file_url, file_name FROM messages ORDER BY id ASC")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var history []Message
		for rows.Next() {
			var m Message
			var sender string
			err := rows.Scan(&m.ID, &sender, &m.Content, &m.Type, &m.FileURL, &m.FileName)
			if err != nil {
				continue
			}
			// В офлайн-режиме sender — это username, кладём в SenderName для отображения
			m.SenderName = sender
			history = append(history, m)
		}
		return history, nil
	}

	// Онлайн
	if a.token == "" {
		return nil, fmt.Errorf("not authenticated")
	}
	url := fmt.Sprintf("%s/api/messages?page=1", a.baseURL)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(body))
	}

	var messages []Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// DeleteMessage удаляет сообщение
func (a *App) DeleteMessage(id int64) error {
	if a.offlineMode {
		_, err := a.db.Exec("DELETE FROM messages WHERE id = ?", id)
		return err
	}
	// Онлайн
	if a.token == "" {
		return fmt.Errorf("not authenticated")
	}
	reqBody := map[string]int64{"id": id}
	data, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", a.baseURL+"/api/messages/delete", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}
	return nil
}

// SelectFile открывает диалог выбора файла и возвращает информацию для отправки
func (a *App) SelectFile() (map[string]interface{}, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Выбрать файл",
		Filters: []runtime.FileFilter{{DisplayName: "Все файлы", Pattern: "*.*"}},
	})
	if err != nil || selection == "" {
		return nil, err
	}

	file, err := os.Open(selection)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	fileName := filepath.Base(selection)
	isImage := strings.HasSuffix(strings.ToLower(fileName), ".jpg") ||
		strings.HasSuffix(strings.ToLower(fileName), ".jpeg") ||
		strings.HasSuffix(strings.ToLower(fileName), ".png") ||
		strings.HasSuffix(strings.ToLower(fileName), ".gif") ||
		strings.HasSuffix(strings.ToLower(fileName), ".webp")

	var fileURL string
	if !a.offlineMode && a.token != "" {
		// Загружаем на сервер
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("voice", fileName)
		part.Write(data)
		writer.Close()

		req, _ := http.NewRequest("POST", a.baseURL+"/api/voice/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", a.token)

		resp, err := a.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("upload failed: %v", result["error"])
		}
		fileURL = result["file_url"].(string)
	} else {
		// Офлайн: встраиваем base64 прямо в URL
		fileURL = "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(data)
	}

	return map[string]interface{}{
		"name":     fileName,
		"url":      fileURL,
		"is_image": isImage,
		"size":     len(data),
	}, nil
}

// ========== WEBSOCKET ==========

func (a *App) connectWebSocket() {
	a.wsMutex.Lock()
	if a.wsConn != nil {
		a.wsConn.Close()
	}
	a.wsMutex.Unlock()

	// Передаём токен в query параметре (можно и в заголовке, но проще так)
	u := fmt.Sprintf("%s/ws?token=%s", strings.Replace(a.baseURL, "http", "ws", 1), a.token)
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		runtime.LogErrorf(a.ctx, "WebSocket connection error: %v", err)
		return
	}
	a.wsMutex.Lock()
	a.wsConn = conn
	a.wsMutex.Unlock()

	go a.readWebSocket()
}

func (a *App) readWebSocket() {
	defer func() {
		a.wsMutex.Lock()
		a.wsConn.Close()
		a.wsConn = nil
		a.wsMutex.Unlock()
	}()

	for {
		var msg Message
		err := a.wsConn.ReadJSON(&msg)
		if err != nil {
			runtime.LogErrorf(a.ctx, "WebSocket read error: %v", err)
			break
		}
		// Эмитим событие для фронтенда
		runtime.EventsEmit(a.ctx, "new_msg", msg)
	}
}
