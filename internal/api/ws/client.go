package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Client represents a single WebSocket connection.
type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewClient upgrades an HTTP connection to WebSocket and returns a Client.
func NewClient(w http.ResponseWriter, r *http.Request) (*Client, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	return &Client{conn: conn}, nil
}

// Send writes a message to the WebSocket connection.
func (c *Client) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the WebSocket connection.
func (c *Client) Close() {
	c.conn.Close()
}

// ReadPump reads messages from the client (handles pings and detects disconnects).
func (c *Client) ReadPump(hub *Hub) {
	defer hub.Unregister(c)

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket: read error: %v", err)
			}
			return
		}
		// We don't process incoming messages — this is broadcast-only
	}
}

// WritePump sends periodic pings to keep the connection alive.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		c.conn.SetWriteDeadline(time.Now().Add(writeWait))
		err := c.conn.WriteMessage(websocket.PingMessage, nil)
		c.mu.Unlock()

		if err != nil {
			return
		}
	}
}
