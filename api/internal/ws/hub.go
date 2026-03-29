package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Event types pushed to clients
const (
	EventNotification    = "notification"
	EventTaskUpdate      = "task_update"
	EventChatMessage     = "chat_message"
	EventAgentStatus     = "agent_status"
	EventHiveStatus      = "hive_status"
	EventHiveDataFlow    = "hive_data_flow"
	EventHiveSprint      = "hive_sprint"
	EventHiveStepUpdate  = "hive_step_update"
	EventMemoryExtracted = "memory_extracted"
)

// Message is the envelope sent over WebSocket
type Message struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// Client represents a single WebSocket connection
type Client struct {
	hub    *Hub
	userID string
	conn   *websocket.Conn
	send   chan []byte
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]bool // userID -> set of clients
	register   chan *Client
	unregister chan *Client
}

var defaultHub *Hub
var once sync.Once

// GetHub returns the singleton Hub instance
func GetHub() *Hub {
	once.Do(func() {
		defaultHub = &Hub{
			clients:    make(map[string]map[*Client]bool),
			register:   make(chan *Client, 64),
			unregister: make(chan *Client, 64),
		}
		go defaultHub.run()
	})
	return defaultHub
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.userID] == nil {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			h.mu.Unlock()
			log.Printf("[ws] client connected: user=%s (total=%d)", client.userID, h.countClients())

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("[ws] client disconnected: user=%s", client.userID)
		}
	}
}

func (h *Hub) countClients() int {
	total := 0
	for _, clients := range h.clients {
		total += len(clients)
	}
	return total
}

// SendToUser sends a message to all connections of a specific user
func (h *Hub) SendToUser(userID string, event string, data interface{}) {
	msg := Message{Event: event, Data: data}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := h.clients[userID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.send <- payload:
		default:
			// Client buffer full, drop message
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(event string, data interface{}) {
	msg := Message{Event: event, Data: data}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.send <- payload:
			default:
			}
		}
	}
}

// HandleWS upgrades an HTTP connection to WebSocket
func HandleWS(hub *Hub, userID string, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:    hub,
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 256),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
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
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
