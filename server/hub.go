package main

import (
	"log"
)

type Hub struct {
	clients       map[*Client]bool
	clientsByUser map[int64]map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	broadcast     chan interface{}
}

func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		clientsByUser: make(map[int64]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan interface{}, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			if client.userID > 0 {
				if h.clientsByUser[client.userID] == nil {
					h.clientsByUser[client.userID] = make(map[*Client]bool)
				}
				h.clientsByUser[client.userID][client] = true
			}
			log.Printf("✅ Client connected. Total: %d", len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if client.userID > 0 && h.clientsByUser[client.userID] != nil {
					delete(h.clientsByUser[client.userID], client)
					if len(h.clientsByUser[client.userID]) == 0 {
						delete(h.clientsByUser, client.userID)
					}
				}
				close(client.send)
				log.Printf("❌ Client disconnected. Total: %d", len(h.clients))
			}

				case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					client.conn.Close()
					// unregister will be called by client's readPump/writePump 
					// or we can call it here but carefully
				}
			}
		}
	}
}


func (h *Hub) SendToUser(userID int64, payload interface{}) {
	if userID <= 0 {
		return
	}
	for c := range h.clientsByUser[userID] {
		select {
		case c.send <- payload:
		default:
			c.conn.Close()
		}
	}
}

