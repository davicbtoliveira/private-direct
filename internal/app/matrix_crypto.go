package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func matrixUserID(username string) string { return "@" + username + ":private-direct" }

func matrixUsername(id string) (string, bool) {
	if !strings.HasPrefix(id, "@") || !strings.HasSuffix(id, ":private-direct") {
		return "", false
	}
	username := strings.TrimSuffix(strings.TrimPrefix(id, "@"), ":private-direct")
	return username, validUsername(username)
}

func (s *Server) handleE2EEKeysUpload(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		DeviceID     string                     `json:"device_id"`
		DeviceKeys   json.RawMessage            `json:"device_keys"`
		OneTimeKeys  map[string]json.RawMessage `json:"one_time_keys"`
		FallbackKeys map[string]json.RawMessage `json:"fallback_keys"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var device struct {
		DeviceID string `json:"device_id"`
	}
	if len(body.DeviceKeys) > 0 && json.Unmarshal(body.DeviceKeys, &device) != nil {
		writeError(w, http.StatusBadRequest, "invalid_device_keys")
		return
	}
	if device.DeviceID == "" {
		device.DeviceID = body.DeviceID
	}
	if device.DeviceID != "" {
		res, err := s.db.ExecContext(r.Context(), `UPDATE e2ee_devices SET public_keys = ?, last_seen_at = ?
			WHERE id = ? AND user_id = ? AND revoked_at IS NULL`, string(body.DeviceKeys), time.Now().UTC().Format(time.RFC3339), device.DeviceID, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "keys_upload_failed")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusForbidden, "unknown_device")
			return
		}
	}
	allKeys := make(map[string]json.RawMessage, len(body.OneTimeKeys)+len(body.FallbackKeys))
	for id, key := range body.OneTimeKeys {
		allKeys[id] = key
	}
	for id, key := range body.FallbackKeys {
		allKeys[id] = key
	}
	if device.DeviceID == "" && len(allKeys) > 0 {
		writeError(w, http.StatusBadRequest, "device_keys_required")
		return
	}
	for id, key := range allKeys {
		if len(key) == 0 || len(key) > 4096 {
			writeError(w, http.StatusBadRequest, "invalid_one_time_key")
			return
		}
		if _, err := s.db.ExecContext(r.Context(), `INSERT OR REPLACE INTO e2ee_one_time_keys
			(user_id, device_id, key_id, key_json) VALUES (?, ?, ?, ?)`, user.ID, device.DeviceID, id, string(key)); err != nil {
			writeError(w, http.StatusInternalServerError, "keys_upload_failed")
			return
		}
	}
	var count int
	_ = s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM e2ee_one_time_keys WHERE user_id = ? AND device_id = ?", user.ID, device.DeviceID).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]any{"one_time_key_counts": map[string]int{"signed_curve25519": count}})
}

func (s *Server) handleE2EEKeysQuery(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		DeviceKeys map[string][]string `json:"device_keys"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	devices := map[string]map[string]json.RawMessage{}
	master, selfSigning, userSigning := map[string]json.RawMessage{}, map[string]json.RawMessage{}, map[string]json.RawMessage{}
	for matrixID, requested := range body.DeviceKeys {
		username, ok := matrixUsername(matrixID)
		if !ok {
			continue
		}
		rows, err := s.db.QueryContext(r.Context(), `SELECT e2ee_devices.id, e2ee_devices.public_keys
			FROM e2ee_devices JOIN users ON users.id=e2ee_devices.user_id
			WHERE users.username=? AND e2ee_devices.revoked_at IS NULL`, username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "keys_query_failed")
			return
		}
		devices[matrixID] = map[string]json.RawMessage{}
		for rows.Next() {
			var id, raw string
			if rows.Scan(&id, &raw) == nil && (len(requested) == 0 || contains(requested, id)) {
				devices[matrixID][id] = json.RawMessage(raw)
			}
		}
		rows.Close()
		var identity string
		if err := s.db.QueryRowContext(r.Context(), `SELECT e2ee_accounts.identity_keys FROM e2ee_accounts JOIN users ON users.id=e2ee_accounts.user_id WHERE users.username=?`, username).Scan(&identity); err == nil {
			var keys struct {
				Master json.RawMessage `json:"master_key"`
				Self   json.RawMessage `json:"self_signing_key"`
				User   json.RawMessage `json:"user_signing_key"`
			}
			if json.Unmarshal([]byte(identity), &keys) == nil {
				if len(keys.Master) > 0 {
					master[matrixID] = keys.Master
				}
				if len(keys.Self) > 0 {
					selfSigning[matrixID] = keys.Self
				}
				if len(keys.User) > 0 {
					userSigning[matrixID] = keys.User
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"device_keys": devices, "master_keys": master, "self_signing_keys": selfSigning, "user_signing_keys": userSigning, "failures": map[string]any{}})
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func (s *Server) handleE2EEKeysClaim(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		OneTimeKeys map[string]map[string]string `json:"one_time_keys"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	result := map[string]map[string]map[string]json.RawMessage{}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "keys_claim_failed")
		return
	}
	defer tx.Rollback()
	for matrixID, devices := range body.OneTimeKeys {
		username, ok := matrixUsername(matrixID)
		if !ok {
			continue
		}
		result[matrixID] = map[string]map[string]json.RawMessage{}
		for deviceID, algorithm := range devices {
			var keyID, keyJSON string
			err := tx.QueryRowContext(r.Context(), `SELECT key_id,key_json FROM e2ee_one_time_keys JOIN users ON users.id=e2ee_one_time_keys.user_id WHERE users.username=? AND device_id=? AND key_id LIKE ? LIMIT 1`, username, deviceID, algorithm+":%").Scan(&keyID, &keyJSON)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				writeError(w, 500, "keys_claim_failed")
				return
			}
			result[matrixID][deviceID] = map[string]json.RawMessage{keyID: json.RawMessage(keyJSON)}
			if _, err = tx.ExecContext(r.Context(), "DELETE FROM e2ee_one_time_keys WHERE device_id=? AND key_id=?", deviceID, keyID); err != nil {
				writeError(w, 500, "keys_claim_failed")
				return
			}
		}
	}
	if err = tx.Commit(); err != nil {
		writeError(w, 500, "keys_claim_failed")
		return
	}
	writeJSON(w, 200, map[string]any{"one_time_keys": result, "failures": map[string]any{}})
}

func (s *Server) handleE2EEToDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	var body struct {
		Messages map[string]map[string]json.RawMessage `json:"messages"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	for matrixID, devices := range body.Messages {
		username, ok := matrixUsername(matrixID)
		if !ok {
			continue
		}
		var recipientID int64
		if s.db.QueryRowContext(r.Context(), "SELECT id FROM users WHERE username=?", username).Scan(&recipientID) != nil {
			continue
		}
		for deviceID, content := range devices {
			targets := []string{deviceID}
			if deviceID == "*" {
				targets = nil
				rows, _ := s.db.QueryContext(r.Context(), "SELECT id FROM e2ee_devices WHERE user_id=? AND revoked_at IS NULL", recipientID)
				if rows != nil {
					for rows.Next() {
						var id string
						if rows.Scan(&id) == nil {
							targets = append(targets, id)
						}
					}
					rows.Close()
				}
			}
			for _, target := range targets {
				_, err := s.db.ExecContext(r.Context(), `INSERT INTO e2ee_to_device_events (recipient_user_id,recipient_device_id,sender,event_type,content,created_at) VALUES (?,?,?,?,?,?)`, recipientID, target, matrixUserID(user.Username), r.PathValue("eventType"), string(content), time.Now().UTC().Format(time.RFC3339))
				if err != nil {
					writeError(w, 500, "to_device_failed")
					return
				}
			}
			s.presence.notifyUser(recipientID, map[string]any{"type": "mailbox_changed"})
		}
	}
	writeJSON(w, 200, map[string]any{})
}

func (s *Server) handleE2EESync(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	since := r.URL.Query().Get("since")
	if deviceID == "" {
		writeError(w, 400, "device_id_required")
		return
	}
	var active int
	if err := s.db.QueryRowContext(r.Context(), `SELECT 1 FROM e2ee_devices WHERE id=? AND user_id=? AND revoked_at IS NULL`, deviceID, user.ID).Scan(&active); err != nil {
		writeError(w, http.StatusForbidden, "device_revoked")
		return
	}
	var cursor int64
	fmt.Sscan(since, &cursor)
	rows, err := s.db.QueryContext(r.Context(), `SELECT sequence,sender,event_type,content FROM e2ee_to_device_events WHERE recipient_user_id=? AND recipient_device_id=? AND sequence>? ORDER BY sequence LIMIT 500`, user.ID, deviceID, cursor)
	if err != nil {
		writeError(w, 500, "sync_failed")
		return
	}
	defer rows.Close()
	events := []map[string]any{}
	next := cursor
	for rows.Next() {
		var seq int64
		var sender, eventType, content string
		if rows.Scan(&seq, &sender, &eventType, &content) != nil {
			continue
		}
		var raw any
		if json.Unmarshal([]byte(content), &raw) != nil {
			continue
		}
		events = append(events, map[string]any{"type": eventType, "sender": sender, "content": raw})
		next = seq
	}
	writeJSON(w, 200, map[string]any{"next": fmt.Sprint(next), "events": events})
}
