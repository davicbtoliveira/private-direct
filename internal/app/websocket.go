package app

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{}

type presenceEvent struct {
	Type   string   `json:"type"`
	User   authUser `json:"user"`
	Online bool     `json:"online"`
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

	if s.presence.add(client) {
		s.broadcastPresence(r.Context(), user, true)
	}
	defer func() {
		if s.presence.remove(client) {
			s.broadcastPresence(context.Background(), user, false)
		}
		conn.Close()
	}()

	for {
		if _, _, err := conn.NextReader(); err != nil {
			return
		}
	}
}

func (s *Server) authenticateWebSocket(r *http.Request) (authUser, bool) {
	if token := r.URL.Query().Get("access_token"); token != "" {
		return s.authenticateToken(token)
	}
	return s.authenticate(r)
}

func (s *Server) broadcastPresence(ctx context.Context, user authUser, online bool) {
	contactIDs, err := s.acceptedContactIDs(ctx, user.ID)
	if err != nil {
		return
	}
	s.presence.sendTo(contactIDs, presenceEvent{
		Type:   "presence",
		User:   user,
		Online: online,
	})
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

	var ids []int64
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
