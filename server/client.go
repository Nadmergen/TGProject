package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan interface{}
	userID   int64
	username string
}

type Message struct {
	Type        string    `json:"type"`
	SenderID    int64     `json:"sender_id"`
	RecipientID int64     `json:"recipient_id"`
	Content     string    `json:"content"`
	MsgType     string    `json:"msg_type"`
	Timestamp   time.Time `json:"timestamp"`
}

type wsInbound struct {
	Event       string          `json:"event"`
	RecipientID int64           `json:"recipient_id,omitempty"`
	ID          int64           `json:"id,omitempty"`
	SDP         json.RawMessage `json:"sdp,omitempty"`
	Candidate   json.RawMessage `json:"candidate,omitempty"`
	CallType    string          `json:"call_type,omitempty"` // voice|video
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var in wsInbound
		err := c.conn.ReadJSON(&in)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ WebSocket error: %v", err)
			}
			break
		}

		switch in.Event {
		case "call_offer", "call_answer", "call_ice", "call_hangup":
			if in.RecipientID <= 0 {
				continue
			}
			c.hub.SendToUser(in.RecipientID, map[string]interface{}{
				"event":       in.Event,
				"sender_id":   c.userID,
				"recipient_id": in.RecipientID,
				"call_type":   in.CallType,
				"sdp":         json.RawMessage(in.SDP),
				"candidate":   json.RawMessage(in.Candidate),
				"id":          in.ID,
				"timestamp":   time.Now(),
			})
		default:
			// keep legacy broadcast path for any other messages
			inMsg := Message{
				Type:        "event",
				SenderID:    c.userID,
				RecipientID: in.RecipientID,
				Timestamp:   time.Now(),
			}
			c.hub.broadcast <- inMsg
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			data, _ := json.Marshal(message)
			w.Write(data)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
