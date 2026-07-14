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
	if err = tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "e2ee_setup_failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"e2ee_ready": true})
}

func (s *Server) handleE2EERecovery(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized"); return }
	var wrapped, salt string
	if err := s.db.QueryRowContext(r.Context(), `SELECT wrapped_master_key,kdf_salt FROM e2ee_accounts WHERE user_id=?`, user.ID).Scan(&wrapped, &salt); err != nil {
		writeError(w, http.StatusNotFound, "e2ee_not_setup"); return
	}
	var backup sql.NullString
	_ = s.db.QueryRowContext(r.Context(), `SELECT ciphertext FROM e2ee_key_backups WHERE user_id=?`, user.ID).Scan(&backup)
	writeJSON(w, http.StatusOK, map[string]any{"wrapped_master_key": wrapped, "kdf_salt": salt, "key_backup": backup.String, "protocol_version": e2eeProtocolVersion})
}

func (s *Server) handleE2EERecoveryDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized"); return }
	var req struct { DeviceID string `json:"device_id"`; DeviceKeys json.RawMessage `json:"device_keys"` }
	if !decodeJSON(w, r, &req) { return }
	if strings.TrimSpace(req.DeviceID) == "" || len(req.DeviceID) > 128 || len(req.DeviceKeys) == 0 || len(req.DeviceKeys) > 32*1024 {
		writeError(w, http.StatusBadRequest, "invalid_recovery_device"); return
	}
	var count int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM e2ee_devices WHERE user_id=? AND revoked_at IS NULL`, user.ID).Scan(&count); err != nil || count >= 10 {
		writeError(w, http.StatusConflict, "device_limit_reached"); return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(r.Context(), `INSERT INTO e2ee_devices (id,user_id,public_keys,created_at,last_seen_at) VALUES (?,?,?,?,?)`, req.DeviceID, user.ID, string(req.DeviceKeys), now, now); err != nil {
		writeError(w, http.StatusConflict, "device_registration_failed"); return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"device_id": req.DeviceID})
}

func (s *Server) handleE2EEKeyBackup(w http.ResponseWriter, r *http.Request) {
	user, ok := s.authenticate(r)
	if !ok { writeError(w, http.StatusUnauthorized, "unauthorized"); return }
	var req struct { Ciphertext string `json:"ciphertext"` }
	if !decodeJSON(w, r, &req) { return }
	if len(req.Ciphertext) == 0 || len(req.Ciphertext) > 8*1024*1024 {
		writeError(w, http.StatusBadRequest, "invalid_key_backup"); return
	}
	_, err := s.db.ExecContext(r.Context(), `INSERT INTO e2ee_key_backups (user_id,ciphertext,updated_at) VALUES (?,?,?) ON CONFLICT(user_id) DO UPDATE SET ciphertext=excluded.ciphertext,updated_at=excluded.updated_at`, user.ID, req.Ciphertext, time.Now().UTC().Format(time.RFC3339))
	if err != nil { writeError(w, http.StatusInternalServerError, "key_backup_failed"); return }
	w.WriteHeader(http.StatusNoContent)
}
