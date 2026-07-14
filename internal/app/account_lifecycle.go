package app

import (
	"database/sql"
	"net/http"
	"time"
)

func (s *Server) handleOperatorPasswordReset(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Operator-Token") != s.cfg.OperatorToken {
		writeError(w, 401, "unauthorized")
		return
	}
	username := normalizeUsername(r.PathValue("username"))
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !validUsername(username) || !validPassword(body.Password) {
		writeError(w, 400, "invalid_password")
		return
	}
	breached, available := s.passwordBreached(r.Context(), body.Password)
	if breached {
		writeError(w, 400, "password_breached")
		return
	}
	hash, err := hashPassword(body.Password)
	if err != nil {
		writeError(w, 500, "password_hash_failed")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "password_reset_failed")
		return
	}
	defer tx.Rollback()
	var id int64
	if tx.QueryRowContext(r.Context(), `SELECT id FROM users WHERE username=?`, username).Scan(&id) != nil {
		writeError(w, 404, "user_not_found")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE users SET password_hash=? WHERE id=?`, string(hash), id); err != nil {
		writeError(w, 500, "password_reset_failed")
		return
	}
	_, _ = tx.ExecContext(r.Context(), `UPDATE refresh_sessions SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`, time.Now().UTC().Format(time.RFC3339), id)
	if tx.Commit() != nil {
		writeError(w, 500, "password_reset_failed")
		return
	}
	s.presence.notifyUser(id, map[string]any{"type": "password_reset"})
	response := map[string]any{"password_reset": true, "message_recovery": "authorized_device_or_recovery_phrase_required"}
	if !available {
		response["warning"] = "password_breach_check_unavailable"
	}
	writeJSON(w, 200, response)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var hash string
	if s.db.QueryRowContext(r.Context(), `SELECT password_hash FROM users WHERE id=?`, user.ID).Scan(&hash) != nil || !verifyPassword(hash, body.Password) {
		writeError(w, 403, "invalid_credentials")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(), `SELECT CASE WHEN user_low_id=? THEN user_high_id ELSE user_low_id END FROM contacts WHERE user_low_id=? OR user_high_id=?`, user.ID, user.ID, user.ID)
	contacts := []int64{}
	if rows != nil {
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				contacts = append(contacts, id)
			}
		}
		rows.Close()
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "account_delete_failed")
		return
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO reserved_usernames(username,reserved_at) VALUES(?,?)`, user.Username, now); err != nil {
		writeError(w, 500, "account_delete_failed")
		return
	}
	statements := []struct {
		q string
		a []any
	}{{`DELETE FROM refresh_sessions WHERE user_id=?`, []any{user.ID}}, {`DELETE FROM encrypted_messages WHERE sender_id=? OR recipient_id=?`, []any{user.ID, user.ID}}, {`DELETE FROM message_tombstones WHERE actor_id=? OR other_user_id=?`, []any{user.ID, user.ID}}, {`DELETE FROM contact_requests WHERE requester_id=? OR recipient_id=?`, []any{user.ID, user.ID}}, {`DELETE FROM contacts WHERE user_low_id=? OR user_high_id=?`, []any{user.ID, user.ID}}, {`DELETE FROM invites WHERE used_by_user_id=?`, []any{user.ID}}, {`DELETE FROM users WHERE id=?`, []any{user.ID}}}
	for _, statement := range statements {
		if _, err = tx.ExecContext(r.Context(), statement.q, statement.a...); err != nil && err != sql.ErrNoRows {
			writeError(w, 500, "account_delete_failed")
			return
		}
	}
	if tx.Commit() != nil {
		writeError(w, 500, "account_delete_failed")
		return
	}
	for _, id := range contacts {
		s.presence.notifyUser(id, map[string]any{"type": "contacts_changed"})
	}
	w.WriteHeader(http.StatusNoContent)
}
