package websocket

import (
    "log"
    "time"
    "github.com/gorilla/websocket"
)

// WebSocket client structure
type WebSocketClient struct {
    conn             *websocket.Conn
    reconnectDelay   time.Duration
    maxReconnectDelay time.Duration
    isConnected      bool
    heartBeatTicker  *time.Ticker
}

// NewWebSocketClient initializes a new WebSocket client
func NewWebSocketClient(url string) (*WebSocketClient, error) {
    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil {
        return nil, err
    }
    client := &WebSocketClient{
        conn:             conn,
        reconnectDelay:   1 * time.Second,
        maxReconnectDelay: 30 * time.Second,
        isConnected:      true,
    }
    client.startHeartbeat()
    return client, nil
}

// startHeartbeat starts a heartbeat mechanism to keep the connection alive
func (c *WebSocketClient) startHeartbeat() {
    c.heartBeatTicker = time.NewTicker(15 * time.Second)
    go func() {
        for range c.heartBeatTicker.C {
            if !c.isConnected {
                break
            }
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                log.Println("Heartbeat failed:", err)
                c.isConnected = false
            }
        }
    }()
}

// Reconnect attempts to reconnect to the WebSocket server using exponential backoff
func (c *WebSocketClient) Reconnect(url string) {
    for !c.isConnected {
        log.Println("Attempting to reconnect...")
        time.Sleep(c.reconnectDelay)
        conn, _, err := websocket.DefaultDialer.Dial(url, nil)
        if err != nil {
            log.Println("Reconnect failed:", err)
            c.reconnectDelay = time.Duration(float64(c.reconnectDelay) * 2)
            if c.reconnectDelay > c.maxReconnectDelay {
                c.reconnectDelay = c.maxReconnectDelay
            }
            continue
        }
        c.conn = conn
        c.isConnected = true
        c.reconnectDelay = 1 * time.Second // reset on success
        log.Println("Reconnected successfully!")
        c.startHeartbeat() // Restart heartbeat mechanism
    }
}

// SendMessage sends a message to the WebSocket server
func (c *WebSocketClient) SendMessage(msg []byte) error {
    if !c.isConnected {
        return fmt.Errorf("Cannot send message: not connected")
    }
    return c.conn.WriteMessage(websocket.TextMessage, msg)
}

// ReceiveMessages listens for messages from the WebSocket server
func (c *WebSocketClient) ReceiveMessages() {
    for {
        _, msg, err := c.conn.ReadMessage()
        if err != nil {
            log.Println("Read message error:", err)
            c.isConnected = false
            go c.Reconnect(c.conn.RemoteAddr().String())
            return
        }
        // Handle incoming messages (e.g., typing indicators, delivery status updates)
        log.Println("Received message:", string(msg))
    }
}

// Close closes the connection
func (c *WebSocketClient) Close() {
    c.heartBeatTicker.Stop()
    c.isConnected = false
    c.conn.Close()
}