package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const maxEncryptedEnvelopeBytes = 24 * 1024

func (s *Server) handleCreateEncryptedMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		ID         string          `json:"id"`
		To         int64           `json:"to"`
		Ciphertext json.RawMessage `json:"ciphertext"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if _, err := uuid.Parse(body.ID); err != nil || body.To <= 0 || len(body.Ciphertext) == 0 || len(body.Ciphertext) > maxEncryptedEnvelopeBytes || !json.Valid(body.Ciphertext) {
		writeError(w, 400, "invalid_message")
		return
	}
	if !s.areContacts(r.Context(), user.ID, body.To) {
		writeError(w, 403, "not_contact")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(r.Context(), `INSERT INTO encrypted_messages (message_id,sender_id,recipient_id,ciphertext,created_at) VALUES (?,?,?,?,?) ON CONFLICT(message_id) DO NOTHING`, body.ID, user.ID, body.To, string(body.Ciphertext), now)
	if err != nil {
		writeError(w, 500, "message_persist_failed")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var sender int64
		if s.db.QueryRowContext(r.Context(), "SELECT sender_id FROM encrypted_messages WHERE message_id=?", body.ID).Scan(&sender) != nil || sender != user.ID {
			writeError(w, 409, "message_id_conflict")
			return
		}
	}
	var sequence int64
	_ = s.db.QueryRowContext(r.Context(), "SELECT sequence FROM encrypted_messages WHERE message_id=?", body.ID).Scan(&sequence)
	writeJSON(w, http.StatusCreated, map[string]any{"id": body.ID, "sequence": sequence, "created_at": now})
}

func (s *Server) handleListEncryptedMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	contactID, err := strconv.ParseInt(r.URL.Query().Get("contact_id"), 10, 64)
	if err != nil || !s.areContacts(r.Context(), user.ID, contactID) {
		writeError(w, 403, "not_contact")
		return
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	query := `SELECT sequence,message_id,sender_id,recipient_id,ciphertext,created_at FROM encrypted_messages WHERE ((sender_id=? AND recipient_id=?) OR (sender_id=? AND recipient_id=?))`
	args := []any{user.ID, contactID, contactID, user.ID}
	if before > 0 {
		query += " AND sequence < ?"
		args = append(args, before)
	}
	query += " ORDER BY sequence DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, 500, "messages_failed")
		return
	}
	defer rows.Close()
	messages := []map[string]any{}
	for rows.Next() {
		var seq, sender, recipient int64
		var id, cipher, created string
		if rows.Scan(&seq, &id, &sender, &recipient, &cipher, &created) != nil {
			continue
		}
		var raw any
		if json.Unmarshal([]byte(cipher), &raw) != nil {
			continue
		}
		messages = append(messages, map[string]any{"sequence": seq, "id": id, "sender_id": sender, "recipient_id": recipient, "ciphertext": raw, "created_at": created})
	}
	writeJSON(w, 200, map[string]any{"messages": messages})
}
