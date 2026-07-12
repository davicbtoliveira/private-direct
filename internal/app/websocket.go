package app

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
)

const webSocketProtocol = "private-direct"

var wsUpgrader = websocket.Upgrader{Subprotocols: []string{webSocketProtocol}}

type presenceSnapshotEvent struct {
	Type        string     `json:"type"`
	OnlineUsers []authUser `json:"online_users"`
}

type presenceEvent struct {
	Type   string   `json:"type"`
	User   authUser `json:"user"`
	Online bool     `json:"online"`
}

type sessionReplacedEvent struct {
	Type string `json:"type"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticateWebSocket(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{user: user, conn: conn}
	client.onWriteFailure = func(failed *wsClient) {
		if s.presence.remove(failed) {
			s.broadcastPresence(context.Background(), failed.user, false)
		}
	}

	_, err = s.presence.activate(client, func() ([]authUser, error) {
		return s.acceptedContacts(r.Context(), user.ID)
	})
	if err != nil {
		client.close()
		return
	}
	defer func() {
		client.close()
		if s.presence.remove(client) {
			s.broadcastPresence(context.Background(), user, false)
		}
	}()

	for {
		var message wsInboundMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if !client.processMessage(func() {
			s.handleWebSocketMessage(r, client, message)
		}) {
			return
		}
	}
}

func (s *Server) authenticateWebSocket(r *http.Request) (authUser, bool) {
	if _, ok := r.URL.Query()["access_token"]; ok {
		return authUser{}, false
	}
	if len(r.Header.Values("Authorization")) != 0 {
		return authUser{}, false
	}
	protocols := websocket.Subprotocols(r)
	if len(protocols) != 2 || protocols[0] != webSocketProtocol || protocols[1] == "" {
		return authUser{}, false
	}
	return s.authenticateToken(protocols[1])
}

func (s *Server) broadcastPresence(ctx context.Context, user authUser, online bool) {
	contactIDs, err := s.acceptedContactIDs(ctx, user.ID)
	if err != nil {
		return
	}
	s.presence.sendPresenceToContacts(user, online, contactIDs)
}

func (s *Server) acceptedContactIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT CASE
		   WHEN contacts.user_low_id = ? THEN contacts.user_high_id
		   ELSE contacts.user_low_id
		 END
		 FROM contacts
		 WHERE contacts.user_low_id = ? OR contacts.user_high_id = ?
		 ORDER BY 1`,
		userID,
		userID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *Server) acceptedContacts(ctx context.Context, userID int64) ([]authUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT users.id, users.username
		 FROM contacts
		 JOIN users ON users.id = CASE
		   WHEN contacts.user_low_id = ? THEN contacts.user_high_id
		   ELSE contacts.user_low_id
		 END
		 WHERE contacts.user_low_id = ? OR contacts.user_high_id = ?
		 ORDER BY users.username`,
		userID,
		userID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	contacts := make([]authUser, 0)
	for rows.Next() {
		var contact authUser
		if err := rows.Scan(&contact.ID, &contact.Username); err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return contacts, nil
}
