package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period (must be < pongWait).
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

// MessageType identifies the kind of real-time message.
type MessageType string

const (
	MsgBuildLog      MessageType = "build_log"
	MsgDeployStatus  MessageType = "deploy_status"
	MsgMetricUpdate  MessageType = "metric_update"
	MsgPipelineEvent MessageType = "pipeline_event"
	MsgSystemAlert   MessageType = "system_alert"
)

// Message is the envelope sent over WebSocket connections.
type Message struct {
	Type      MessageType    `json:"type"`
	Room      string         `json:"room"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time      `json:"timestamp"`
}

// BuildLogPayload is the payload for build log messages.
type BuildLogPayload struct {
	BuildID string `json:"build_id"`
	Line    string `json:"line"`
	Stream  string `json:"stream"` // stdout or stderr
	Step    string `json:"step"`
}

// DeployStatusPayload is the payload for deployment status updates.
type DeployStatusPayload struct {
	DeploymentID string `json:"deployment_id"`
	Environment  string `json:"environment"`
	Status       string `json:"status"` // pending, running, succeeded, failed
	Message      string `json:"message"`
	Replicas     int32  `json:"replicas"`
	Ready        int32  `json:"ready"`
}

// MetricUpdatePayload is the payload for metric updates pushed to dashboards.
type MetricUpdatePayload struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
	Timestamp time.Time         `json:"timestamp"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	rooms  map[string]bool
	mu     sync.RWMutex
	userID string
}

// NewClient wraps a WebSocket connection into a Client.
func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		userID: userID,
	}
}

// UserID returns the authenticated user ID for the client.
func (c *Client) UserID() string {
	return c.userID
}

// Rooms returns a copy of the room names the client is subscribed to.
func (c *Client) Rooms() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rooms := make([]string, 0, len(c.rooms))
	for r := range c.rooms {
		rooms = append(rooms, r)
	}
	return rooms
}

// ReadPump reads messages from the WebSocket connection and dispatches them.
// It must be called in its own goroutine per client.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ws: read error for client %s: %v", c.userID, err)
			}
			break
		}

		// Parse incoming message to check for room join/leave commands.
		var msg struct {
			Action string `json:"action"`
			Room   string `json:"room"`
		}
		if err := json.Unmarshal(message, &msg); err == nil {
			switch msg.Action {
			case "join":
				c.hub.JoinRoom(c, msg.Room)
				continue
			case "leave":
				c.hub.LeaveRoom(c, msg.Room)
				continue
			}
		}

		// Broadcast non-command messages to the client's rooms.
		c.mu.RLock()
		for room := range c.rooms {
			c.hub.BroadcastToRoom(room, message)
		}
		c.mu.RUnlock()
	}
}

// WritePump sends messages from the send channel to the WebSocket connection.
// It must be called in its own goroutine per client.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Drain any queued messages into the same write.
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte("\n"))
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub maintains the set of active clients and broadcasts messages to rooms.
type Hub struct {
	clients    map[*Client]bool
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message
	mu         sync.RWMutex
}

// NewHub creates a new Hub ready to accept clients.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message, 256),
	}
}

// Run starts the hub's main event loop. It should be called in its own goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("ws: client registered (user=%s, total=%d)", client.userID, len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)

				// Remove from all rooms.
				client.mu.RLock()
				for room := range client.rooms {
					if members, ok := h.rooms[room]; ok {
						delete(members, client)
						if len(members) == 0 {
							delete(h.rooms, room)
						}
					}
				}
				client.mu.RUnlock()
			}
			h.mu.Unlock()
			log.Printf("ws: client unregistered (user=%s, total=%d)", client.userID, len(h.clients))

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg)
			if err != nil {
				log.Printf("ws: failed to marshal broadcast message: %v", err)
				continue
			}
			h.mu.RLock()
			if members, ok := h.rooms[msg.Room]; ok {
				for client := range members {
					select {
					case client.send <- data:
					default:
						// Client's buffer is full; drop it.
						go func(c *Client) { h.unregister <- c }(client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// RegisterClient adds a client to the hub.
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient removes a client from the hub.
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// BroadcastToRoom sends a raw byte message to every client in the room.
func (h *Hub) BroadcastToRoom(room string, message []byte) {
	h.mu.RLock()
	members, ok := h.rooms[room]
	h.mu.RUnlock()
	if !ok {
		return
	}

	h.mu.RLock()
	for client := range members {
		select {
		case client.send <- message:
		default:
			go func(c *Client) { h.unregister <- c }(client)
		}
	}
	h.mu.RUnlock()
}

// BroadcastMessage marshals and broadcasts a structured Message to a room.
func (h *Hub) BroadcastMessage(msg *Message) {
	msg.Timestamp = time.Now().UTC()
	h.broadcast <- msg
}

// JoinRoom adds a client to a named room.
func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true

	client.mu.Lock()
	client.rooms[room] = true
	client.mu.Unlock()

	log.Printf("ws: client %s joined room %s", client.userID, room)
}

// LeaveRoom removes a client from a named room.
func (h *Hub) LeaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if members, ok := h.rooms[room]; ok {
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}

	client.mu.Lock()
	delete(client.rooms, room)
	client.mu.Unlock()

	log.Printf("ws: client %s left room %s", client.userID, room)
}

// RoomSize returns the number of clients currently in a room.
func (h *Hub) RoomSize(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if members, ok := h.rooms[room]; ok {
		return len(members)
	}
	return 0
}

// ClientCount returns the total number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// HandleWebSocket upgrades an HTTP connection to a WebSocket, creates a
// Client, and registers it with the Hub.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade error: %v", err)
		return
	}
	userID := r.URL.Query().Get("user_id")
	client := NewClient(h, conn, userID)
	h.RegisterClient(client)
	go client.WritePump()
	go client.ReadPump()
}
