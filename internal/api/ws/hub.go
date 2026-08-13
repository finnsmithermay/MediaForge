package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// Event represents a WebSocket message sent to clients.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// JobCompleteData is the payload for job completion events.
type JobCompleteData struct {
	JobID        string `json:"job_id"`
	Status       string `json:"status"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Hub manages active WebSocket connections and broadcasts messages.
type Hub struct {
	clients map[*Client]bool
	mu      sync.RWMutex
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
	log.Printf("WebSocket: client connected (%d total)", len(h.clients))
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.Close()
		log.Printf("WebSocket: client disconnected (%d total)", len(h.clients))
	}
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("WebSocket: failed to marshal event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := client.Send(data); err != nil {
			log.Printf("WebSocket: failed to send to client: %v", err)
			go h.Unregister(client)
		}
	}
}

// BroadcastJobComplete sends a job completion notification.
func (h *Hub) BroadcastJobComplete(jobID, status, thumbnailURL, jobError string) {
	h.Broadcast(Event{
		Type: "job_complete",
		Data: JobCompleteData{
			JobID:        jobID,
			Status:       status,
			ThumbnailURL: thumbnailURL,
			Error:        jobError,
		},
	})
}
