package app

import (
	"encoding/json"
	"net/http"
)

type wsInboundMessage struct {
	Type       string          `json:"type"`
	ToUserID   int64           `json:"to_user_id"`
	SignalType string          `json:"signal_type"`
	Payload    json.RawMessage `json:"payload"`
}

type signalEvent struct {
	Type       string          `json:"type"`
	From       authUser        `json:"from"`
	SignalType string          `json:"signal_type"`
	Payload    json.RawMessage `json:"payload"`
}

type wsErrorEvent struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

func (s *Server) handleWebSocketMessage(r *http.Request, client *wsClient, message wsInboundMessage) {
	if message.Type != "signal" {
		client.writeJSON(wsErrorEvent{Type: "error", Error: "unsupported_message_type"})
		return
	}
	if message.ToUserID == 0 {
		client.writeJSON(wsErrorEvent{Type: "error", Error: "target_required"})
		return
	}
	if !validSignalType(message.SignalType) {
		client.writeJSON(wsErrorEvent{Type: "error", Error: "invalid_signal_type"})
		return
	}
	if !s.areContacts(r.Context(), client.user.ID, message.ToUserID) {
		client.writeJSON(wsErrorEvent{Type: "error", Error: "not_contact"})
		return
	}
	if len(message.Payload) == 0 {
		message.Payload = json.RawMessage("null")
	}

	sent := s.presence.sendToUser(message.ToUserID, signalEvent{
		Type:       "signal",
		From:       client.user,
		SignalType: message.SignalType,
		Payload:    message.Payload,
	})
	if !sent {
		client.writeJSON(wsErrorEvent{Type: "error", Error: "target_offline"})
	}
}

func validSignalType(signalType string) bool {
	return signalType == "offer" || signalType == "answer" || signalType == "ice"
}
