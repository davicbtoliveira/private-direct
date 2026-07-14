package app

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type messageRate struct {
	tokens  float64
	updated time.Time
}

func (s *Server) allowMessage(userID int64) bool {
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	now := time.Now()
	bucket := s.messageRates[userID]
	if bucket == nil {
		bucket = &messageRate{tokens: float64(s.cfg.MessageRateBurst), updated: now}
		s.messageRates[userID] = bucket
	}
	elapsed := now.Sub(bucket.updated).Minutes()
	bucket.tokens += elapsed * float64(s.cfg.MessageRatePerMinute)
	if bucket.tokens > float64(s.cfg.MessageRateBurst) {
		bucket.tokens = float64(s.cfg.MessageRateBurst)
	}
	bucket.updated = now
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

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
	var deleted int
	_ = s.db.QueryRowContext(r.Context(), `SELECT 1 FROM message_tombstones WHERE message_id=? AND scope='both' LIMIT 1`, body.ID).Scan(&deleted)
	if deleted == 1 {
		writeError(w, http.StatusGone, "message_deleted")
		return
	}
	var existingSender, existingSequence int64
	if err := s.db.QueryRowContext(r.Context(), "SELECT sender_id,sequence FROM encrypted_messages WHERE message_id=?", body.ID).Scan(&existingSender, &existingSequence); err == nil {
		if existingSender != user.ID {
			writeError(w, 409, "message_id_conflict")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": body.ID, "sequence": existingSequence})
		return
	}
	if !s.allowMessage(user.ID) {
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusTooManyRequests, "message_rate_limited")
		return
	}
	if !s.areContacts(r.Context(), user.ID, body.To) {
		writeError(w, 403, "not_contact")
		return
	}
	if s.cfg.MessageQuotaBytes > 0 {
		var used int64
		_ = s.db.QueryRowContext(r.Context(), "SELECT COALESCE(SUM(LENGTH(ciphertext)),0) FROM encrypted_messages WHERE sender_id=?", user.ID).Scan(&used)
		if used+int64(len(body.Ciphertext)) > s.cfg.MessageQuotaBytes {
			writeError(w, http.StatusInsufficientStorage, "message_quota_exceeded")
			return
		}
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
	s.presence.notifyUser(body.To, map[string]any{"type": "mailbox_changed", "cursor": sequence})
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
	query := `SELECT sequence,message_id,sender_id,recipient_id,ciphertext,created_at,EXISTS(SELECT 1 FROM message_deliveries WHERE message_deliveries.message_id=encrypted_messages.message_id) FROM encrypted_messages WHERE ((sender_id=? AND recipient_id=?) OR (sender_id=? AND recipient_id=?)) AND NOT EXISTS(SELECT 1 FROM message_tombstones t WHERE t.message_id=encrypted_messages.message_id AND (t.scope='both' OR (t.scope='self' AND t.actor_id=?)))`
	args := []any{user.ID, contactID, contactID, user.ID, user.ID}
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
		var delivered bool
		if rows.Scan(&seq, &id, &sender, &recipient, &cipher, &created, &delivered) != nil {
			continue
		}
		var raw any
		if json.Unmarshal([]byte(cipher), &raw) != nil {
			continue
		}
		messages = append(messages, map[string]any{"sequence": seq, "id": id, "sender_id": sender, "recipient_id": recipient, "ciphertext": raw, "created_at": created, "delivered": delivered})
	}
	deleted := []string{}
	tombstones, _ := s.db.QueryContext(r.Context(), `SELECT DISTINCT message_id FROM message_tombstones WHERE ((actor_id=? AND other_user_id=?) OR (actor_id=? AND other_user_id=?)) AND (scope='both' OR actor_id=?)`, user.ID, contactID, contactID, user.ID, user.ID)
	if tombstones != nil {
		defer tombstones.Close()
		for tombstones.Next() {
			var id string
			if tombstones.Scan(&id) == nil {
				deleted = append(deleted, id)
			}
		}
	}
	writeJSON(w, 200, map[string]any{"messages": messages, "deleted": deleted})
}

func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Scope     string `json:"scope"`
		DeviceID  string `json:"device_id"`
		CreatedAt string `json:"created_at"`
		Signature string `json:"signature"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Scope != "self" && body.Scope != "both" {
		writeError(w, 400, "invalid_delete_scope")
		return
	}
	var sender, recipient int64
	if s.db.QueryRowContext(r.Context(), `SELECT sender_id,recipient_id FROM encrypted_messages WHERE message_id=?`, id).Scan(&sender, &recipient) != nil {
		if s.db.QueryRowContext(r.Context(), `SELECT actor_id,other_user_id FROM message_tombstones WHERE message_id=? AND scope='both' LIMIT 1`, id).Scan(&sender, &recipient) != nil {
			writeError(w, 404, "message_not_found")
			return
		}
	}
	if user.ID != sender && user.ID != recipient {
		writeError(w, 403, "not_participant")
		return
	}
	other := sender
	if other == user.ID {
		other = recipient
	}
	var raw string
	if s.db.QueryRowContext(r.Context(), `SELECT public_keys FROM e2ee_devices WHERE id=? AND user_id=? AND revoked_at IS NULL`, body.DeviceID, user.ID).Scan(&raw) != nil {
		writeError(w, 403, "unknown_device")
		return
	}
	var keys struct {
		Keys map[string]string `json:"keys"`
	}
	if json.Unmarshal([]byte(raw), &keys) != nil {
		writeError(w, 400, "invalid_device_key")
		return
	}
	publicRaw, err := base64.RawStdEncoding.DecodeString(keys.Keys["ed25519:"+body.DeviceID])
	if err != nil {
		publicRaw, err = base64.StdEncoding.DecodeString(keys.Keys["ed25519:"+body.DeviceID])
	}
	signature, err2 := base64.RawStdEncoding.DecodeString(body.Signature)
	if err2 != nil {
		signature, err2 = base64.StdEncoding.DecodeString(body.Signature)
	}
	signed := id + "|" + body.Scope + "|" + body.CreatedAt
	if err != nil || err2 != nil || len(publicRaw) != ed25519.PublicKeySize || !ed25519.Verify(ed25519.PublicKey(publicRaw), []byte(signed), signature) {
		writeError(w, 403, "invalid_tombstone_signature")
		return
	}
	_, err = s.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO message_tombstones(message_id,actor_id,other_user_id,scope,device_id,signature,created_at) VALUES(?,?,?,?,?,?,?)`, id, user.ID, other, body.Scope, body.DeviceID, body.Signature, body.CreatedAt)
	if err != nil {
		writeError(w, 500, "delete_failed")
		return
	}
	if body.Scope == "both" {
		_, _ = s.db.ExecContext(r.Context(), `DELETE FROM encrypted_messages WHERE message_id=?`, id)
		s.presence.notifyUser(other, map[string]any{"type": "mailbox_changed"})
	}
	s.presence.notifyUser(user.ID, map[string]any{"type": "mailbox_changed"})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationRead(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	contactID, err := strconv.ParseInt(r.PathValue("contactID"), 10, 64)
	if err != nil || !s.areContacts(r.Context(), user.ID, contactID) {
		writeError(w, 403, "not_contact")
		return
	}
	var body struct {
		Sequence int64 `json:"sequence"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var max int64
	_ = s.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(sequence),0) FROM encrypted_messages WHERE (sender_id=? AND recipient_id=?) OR (sender_id=? AND recipient_id=?)`, user.ID, contactID, contactID, user.ID).Scan(&max)
	if body.Sequence < 0 || body.Sequence > max {
		writeError(w, 400, "invalid_read_cursor")
		return
	}
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO conversation_reads(user_id,contact_id,last_read_sequence,updated_at) VALUES(?,?,?,?) ON CONFLICT(user_id,contact_id) DO UPDATE SET last_read_sequence=MAX(last_read_sequence,excluded.last_read_sequence),updated_at=excluded.updated_at`, user.ID, contactID, body.Sequence, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		writeError(w, 500, "read_cursor_failed")
		return
	}
	s.presence.notifyUser(user.ID, map[string]any{"type": "read_state_changed", "contact_id": contactID, "sequence": body.Sequence})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnreadCounts(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT sender_id,COUNT(*) FROM encrypted_messages m WHERE recipient_id=? AND sequence>COALESCE((SELECT last_read_sequence FROM conversation_reads WHERE user_id=? AND contact_id=m.sender_id),0) GROUP BY sender_id`, user.ID, user.ID)
	if err != nil {
		writeError(w, 500, "unread_failed")
		return
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var id int64
		var count int
		if rows.Scan(&id, &count) == nil {
			counts[strconv.FormatInt(id, 10)] = count
		}
	}
	writeJSON(w, 200, map[string]any{"unread": counts})
}

func (s *Server) handleMessageDelivered(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	id := r.PathValue("id")
	var sender, recipient int64
	if s.db.QueryRowContext(r.Context(), "SELECT sender_id,recipient_id FROM encrypted_messages WHERE message_id=?", id).Scan(&sender, &recipient) != nil {
		writeError(w, 404, "message_not_found")
		return
	}
	if recipient != user.ID {
		writeError(w, 403, "not_recipient")
		return
	}
	if _, err := s.db.ExecContext(r.Context(), "INSERT OR IGNORE INTO message_deliveries(message_id,delivered_at) VALUES(?,?)", id, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		writeError(w, 500, "delivery_failed")
		return
	}
	s.presence.notifyUser(sender, map[string]any{"type": "mailbox_changed"})
	w.WriteHeader(http.StatusNoContent)
}
