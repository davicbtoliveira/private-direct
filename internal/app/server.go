package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	cfg      Config
	db       *sql.DB
	mux      *http.ServeMux
	presence *presenceHub
}

func NewServer(cfg Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := openDB(context.Background(), cfg.DatabasePath)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:      cfg,
		db:       db,
		mux:      http.NewServeMux(),
		presence: newPresenceHub(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /operator/invites", s.handleCreateInvite)
	s.mux.HandleFunc("POST /register", s.handleRegister)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /refresh", s.handleRefresh)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("GET /users/lookup", s.handleLookupUser)
	s.mux.HandleFunc("POST /contacts/requests", s.handleCreateContactRequest)
	s.mux.HandleFunc("GET /contacts/requests/incoming", s.handleIncomingContactRequests)
	s.mux.HandleFunc("POST /contacts/requests/{id}/accept", s.handleAcceptContactRequest)
	s.mux.HandleFunc("POST /contacts/requests/{id}/reject", s.handleRejectContactRequest)
	s.mux.HandleFunc("GET /contacts", s.handleListContacts)
	s.mux.HandleFunc("GET /ice-servers", s.handleICEServers)
	s.mux.HandleFunc("GET /ws", s.handleWebSocket)
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
	username := strings.TrimSpace(req.Username)
	if inviteCode == "" {
		writeError(w, http.StatusBadRequest, "invite_code_required")
		return
	}
	if username == "" {
		writeError(w, http.StatusBadRequest, "username_required")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password_required")
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
	err = tx.QueryRowContext(r.Context(),
		"SELECT id FROM invites WHERE code = ? AND used_by_user_id IS NULL",
		inviteCode,
	).Scan(&inviteID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusBadRequest, "invalid_invite")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "register_failed")
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

	writeJSON(w, http.StatusCreated, map[string]any{"id": userID, "username": username})
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
