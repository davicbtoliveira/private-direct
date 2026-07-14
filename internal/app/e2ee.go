package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const e2eeProtocolVersion = 1

func (s *Server) handleE2EESetup(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req struct {
		DeviceID         string          `json:"device_id"`
		DeviceName       string          `json:"device_name"`
		IdentityKeys     json.RawMessage `json:"identity_keys"`
		DeviceKeys       json.RawMessage `json:"device_keys"`
		WrappedMasterKey string          `json:"wrapped_master_key"`
		KDFSalt          string          `json:"kdf_salt"`
		ProtocolVersion  int             `json:"protocol_version"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProtocolVersion != e2eeProtocolVersion ||
		strings.TrimSpace(req.DeviceID) == "" || len(req.DeviceID) > 128 ||
		len(req.IdentityKeys) == 0 || len(req.IdentityKeys) > 32*1024 ||
		len(req.DeviceKeys) == 0 || len(req.DeviceKeys) > 32*1024 ||
		len(req.WrappedMasterKey) == 0 || len(req.WrappedMasterKey) > 4096 ||
		len(req.KDFSalt) == 0 || len(req.KDFSalt) > 512 {
		writeError(w, http.StatusBadRequest, "invalid_e2ee_setup")
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(r.Context(), "SELECT 1 FROM e2ee_accounts WHERE user_id = ?", user.ID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	if err == nil {
		writeError(w, http.StatusConflict, "e2ee_already_setup")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO e2ee_accounts
		(user_id, protocol_version, identity_keys, wrapped_master_key, kdf_salt, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, user.ID, req.ProtocolVersion, string(req.IdentityKeys), req.WrappedMasterKey, req.KDFSalt, now); err != nil {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO e2ee_devices
		(id, user_id, public_keys, created_at, last_seen_at) VALUES (?, ?, ?, ?, ?)`,
		req.DeviceID, user.ID, string(req.DeviceKeys), now, now); err != nil {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	name := strings.TrimSpace(req.DeviceName)
	if name == "" || len(name) > 100 {
		name = "Browser"
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO e2ee_device_names (device_id,display_name) VALUES (?,?)`, req.DeviceID, name); err != nil {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"e2ee_ready": true})
}

func (s *Server) handleE2EERecovery(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var wrapped, salt string
	if err := s.db.QueryRowContext(r.Context(), `SELECT wrapped_master_key,kdf_salt FROM e2ee_accounts WHERE user_id=?`, user.ID).Scan(&wrapped, &salt); err != nil {
		writeError(w, http.StatusNotFound, "e2ee_not_setup")
		return
	}
	var backup sql.NullString
	_ = s.db.QueryRowContext(r.Context(), `SELECT ciphertext FROM e2ee_key_backups WHERE user_id=?`, user.ID).Scan(&backup)
	var identity string
	_ = s.db.QueryRowContext(r.Context(), `SELECT identity_keys FROM e2ee_accounts WHERE user_id=?`, user.ID).Scan(&identity)
	writeJSON(w, http.StatusOK, map[string]any{"wrapped_master_key": wrapped, "kdf_salt": salt, "key_backup": backup.String, "identity_keys": json.RawMessage(identity), "protocol_version": e2eeProtocolVersion})
}

func (s *Server) handleE2EERecoveryDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req struct {
		DeviceID   string          `json:"device_id"`
		DeviceName string          `json:"device_name"`
		DeviceKeys json.RawMessage `json:"device_keys"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.DeviceID) == "" || len(req.DeviceID) > 128 || len(req.DeviceKeys) == 0 || len(req.DeviceKeys) > 32*1024 {
		writeError(w, http.StatusBadRequest, "invalid_recovery_device")
		return
	}
	var count int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM e2ee_devices WHERE user_id=? AND revoked_at IS NULL`, user.ID).Scan(&count); err != nil || count >= 10 {
		writeError(w, http.StatusConflict, "device_limit_reached")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO e2ee_devices (id,user_id,public_keys,created_at,last_seen_at) VALUES (?,?,?,?,?)`, req.DeviceID, user.ID, string(req.DeviceKeys), now, now); err != nil {
		writeError(w, http.StatusConflict, "device_registration_failed")
		return
	}
	name := strings.TrimSpace(req.DeviceName)
	if name == "" || len(name) > 100 {
		name = "Browser"
	}
	_, _ = s.db.ExecContext(r.Context(), `INSERT INTO e2ee_device_names (device_id,display_name) VALUES (?,?)`, req.DeviceID, name)
	writeJSON(w, http.StatusCreated, map[string]any{"device_id": req.DeviceID})
}

func (s *Server) handleE2EEDevices(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT d.id,COALESCE(n.display_name,'Browser'),d.created_at,d.last_seen_at FROM e2ee_devices d LEFT JOIN e2ee_device_names n ON n.device_id=d.id WHERE d.user_id=? AND d.revoked_at IS NULL ORDER BY d.created_at`, user.ID)
	if err != nil {
		writeError(w, 500, "devices_failed")
		return
	}
	defer rows.Close()
	devices := []map[string]any{}
	for rows.Next() {
		var id, name, created, last string
		if rows.Scan(&id, &name, &created, &last) == nil {
			devices = append(devices, map[string]any{"id": id, "name": name, "created_at": created, "last_seen_at": last})
		}
	}
	writeJSON(w, 200, map[string]any{"devices": devices, "limit": 10})
}

func (s *Server) handleE2EEDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	res, err := s.db.ExecContext(r.Context(), `UPDATE e2ee_devices SET revoked_at=? WHERE id=? AND user_id=? AND revoked_at IS NULL`, time.Now().UTC().Format(time.RFC3339), r.PathValue("id"), user.ID)
	if err != nil {
		writeError(w, 500, "device_revoke_failed")
		return
	}
	changed, _ := res.RowsAffected()
	if changed == 0 {
		writeError(w, 404, "device_not_found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleE2EEContactIdentity(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	username := normalizeUsername(r.PathValue("username"))
	var contactID int64
	var identity string
	err := s.db.QueryRowContext(r.Context(), `SELECT users.id,e2ee_accounts.identity_keys FROM users JOIN e2ee_accounts ON e2ee_accounts.user_id=users.id WHERE users.username=?`, username).Scan(&contactID, &identity)
	if err != nil || !s.areContacts(r.Context(), user.ID, contactID) {
		writeError(w, 404, "contact_identity_not_found")
		return
	}
	writeJSON(w, 200, map[string]any{"username": username, "identity_keys": json.RawMessage(identity)})
}

func (s *Server) handleE2EEKeyBackup(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req struct {
		Ciphertext string `json:"ciphertext"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Ciphertext) == 0 || len(req.Ciphertext) > 8*1024*1024 {
		writeError(w, http.StatusBadRequest, "invalid_key_backup")
		return
	}
	_, err := s.db.ExecContext(r.Context(), `INSERT INTO e2ee_key_backups (user_id,ciphertext,updated_at) VALUES (?,?,?) ON CONFLICT(user_id) DO UPDATE SET ciphertext=excluded.ciphertext,updated_at=excluded.updated_at`, user.ID, req.Ciphertext, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key_backup_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
