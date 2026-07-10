package app

import (
	"sync"

	"github.com/gorilla/websocket"
)

type presenceHub struct {
	mu      sync.RWMutex
	clients map[int64]map[*wsClient]struct{}
}

type wsClient struct {
	user authUser
	conn *websocket.Conn
	mu   sync.Mutex
}

func newPresenceHub() *presenceHub {
	return &presenceHub{clients: map[int64]map[*wsClient]struct{}{}}
}

func (h *presenceHub) add(client *wsClient) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := h.clients[client.user.ID]
	first := len(clients) == 0
	if clients == nil {
		clients = map[*wsClient]struct{}{}
		h.clients[client.user.ID] = clients
	}
	clients[client] = struct{}{}
	return first
}

func (h *presenceHub) remove(client *wsClient) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := h.clients[client.user.ID]
	if clients == nil {
		return false
	}
	delete(clients, client)
	if len(clients) > 0 {
		return false
	}
	delete(h.clients, client.user.ID)
	return true
}

func (h *presenceHub) sendTo(userIDs []int64, message any) {
	h.mu.RLock()
	var targets []*wsClient
	for _, userID := range userIDs {
		for client := range h.clients[userID] {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	for _, target := range targets {
		target.writeJSON(message)
	}
}

func (h *presenceHub) sendToUser(userID int64, message any) bool {
	h.mu.RLock()
	var targets []*wsClient
	for client := range h.clients[userID] {
		targets = append(targets, client)
	}
	h.mu.RUnlock()

	for _, target := range targets {
		target.writeJSON(message)
	}
	return len(targets) > 0
}

func (c *wsClient) writeJSON(message any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.WriteJSON(message)
}
