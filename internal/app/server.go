package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/davicbtoliveira/private-direct/web"
)

type Server struct {
	cfg          Config
	db           *sql.DB
	mux          *http.ServeMux
	presence     *presenceHub
	dist         fs.FS
	httpClient   *http.Client
	rateMu       sync.Mutex
	messageRates map[int64]*messageRate
	backup       *backupManager
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.MessageQuotaBytes == 0 {
		cfg.MessageQuotaBytes = 100 * 1024 * 1024
	}
	if cfg.MessageRatePerMinute == 0 {
		cfg.MessageRatePerMinute = 120
	}
	if cfg.MessageRateBurst == 0 {
		cfg.MessageRateBurst = 30
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := openDB(context.Background(), cfg.DatabasePath)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:          cfg,
		db:           db,
		mux:          http.NewServeMux(),
		presence:     newPresenceHub(),
		httpClient:   &http.Client{Timeout: 3 * time.Second},
		messageRates: make(map[int64]*messageRate),
	}
	s.dist, err = fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("embed spa: %w", err)
	}
	s.routes()
	if cfg.BackupDirectory != "" {
		s.backup, err = newBackupManager(s)
		if err != nil {
			db.Close()
			return nil, err
		}
		s.backup.start()
	}
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			s.mux.ServeHTTP(w, r)
			return
		}
		s.serveSPA(w, r)
	})
}

func (s *Server) Close() error {
	if s.backup != nil {
		s.backup.close()
	}
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/operator/backup/health", s.handleBackupHealth)
	s.mux.HandleFunc("POST /api/operator/invites", s.handleCreateInvite)
	s.mux.HandleFunc("POST /api/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/refresh", s.handleRefresh)
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)
	s.mux.HandleFunc("POST /api/e2ee/setup", s.handleE2EESetup)
	s.mux.HandleFunc("GET /api/e2ee/recovery", s.handleE2EERecovery)
	s.mux.HandleFunc("POST /api/e2ee/recovery/devices", s.handleE2EERecoveryDevice)
	s.mux.HandleFunc("PUT /api/e2ee/key-backup", s.handleE2EEKeyBackup)
	s.mux.HandleFunc("POST /api/e2ee/keys/upload", s.handleE2EEKeysUpload)
	s.mux.HandleFunc("POST /api/e2ee/keys/query", s.handleE2EEKeysQuery)
	s.mux.HandleFunc("POST /api/e2ee/keys/claim", s.handleE2EEKeysClaim)
	s.mux.HandleFunc("POST /api/e2ee/to-device/{eventType}/{txnID}", s.handleE2EEToDevice)
	s.mux.HandleFunc("GET /api/e2ee/sync", s.handleE2EESync)
	s.mux.HandleFunc("POST /api/messages", s.handleCreateEncryptedMessage)
	s.mux.HandleFunc("GET /api/messages", s.handleListEncryptedMessages)
	s.mux.HandleFunc("POST /api/messages/{id}/delivered", s.handleMessageDelivered)
	s.mux.HandleFunc("GET /api/users/lookup", s.handleLookupUser)
	s.mux.HandleFunc("POST /api/contacts/requests", s.handleCreateContactRequest)
	s.mux.HandleFunc("GET /api/contacts/requests/incoming", s.handleIncomingContactRequests)
	s.mux.HandleFunc("POST /api/contacts/requests/{id}/accept", s.handleAcceptContactRequest)
	s.mux.HandleFunc("POST /api/contacts/requests/{id}/reject", s.handleRejectContactRequest)
	s.mux.HandleFunc("GET /api/contacts", s.handleListContacts)
	s.mux.HandleFunc("GET /api/ice-servers", s.handleICEServers)
	s.mux.HandleFunc("GET /api/ws", s.handleWebSocket)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var version int
	if err := s.db.QueryRowContext(r.Context(), "SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, fmt.Sprintf("encode response: %v", err), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Operator-Token") != s.cfg.OperatorToken {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "code_required")
		return
	}

	_, err := s.db.ExecContext(r.Context(),
		"INSERT INTO invites (code, created_at) VALUES (?, ?)",
		code,
		time.Now().UTC().Format(time.RFC3339),
	)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "invite_exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_invite_failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"code": code})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"invite_code"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	inviteCode := strings.TrimSpace(req.InviteCode)
	username := normalizeUsername(req.Username)
	if inviteCode == "" {
		writeError(w, http.StatusBadRequest, "invite_code_required")
		return
	}
	if username == "" {
		writeError(w, http.StatusBadRequest, "username_required")
		return
	}
	if !validUsername(username) {
		writeError(w, http.StatusBadRequest, "invalid_username")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password_required")
		return
	}
	if !validPassword(req.Password) {
		writeError(w, http.StatusBadRequest, "invalid_password")
		return
	}

	breached, breachCheckAvailable := s.passwordBreached(r.Context(), req.Password)
	if breached {
		writeError(w, http.StatusBadRequest, "password_breached")
		return
	}

	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "password_hash_failed")
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
		return
	}
	defer tx.Rollback()

	var inviteID int64
	var inviteUsedBy sql.NullInt64
	err = tx.QueryRowContext(r.Context(),
		"SELECT id, used_by_user_id FROM invites WHERE code = ?",
		inviteCode,
	).Scan(&inviteID, &inviteUsedBy)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusBadRequest, "invalid_invite")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
		return
	}
	if inviteUsedBy.Valid {
		writeError(w, http.StatusBadRequest, "invite_used")
		return
	}

	res, err := tx.ExecContext(r.Context(),
		"INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)",
		username,
		string(passwordHash),
		time.Now().UTC().Format(time.RFC3339),
	)
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "username_exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
		return
	}
	userID, err := res.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
		return
	}

	if _, err := tx.ExecContext(r.Context(),
		"UPDATE invites SET used_by_user_id = ?, used_at = ? WHERE id = ?",
		userID,
		time.Now().UTC().Format(time.RFC3339),
		inviteID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
		return
	}

	response := map[string]any{"id": userID, "username": username}
	if !breachCheckAvailable {
		response["warning"] = "password_breach_check_unavailable"
	}
	writeJSON(w, http.StatusCreated, response)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "constraint failed")
}
