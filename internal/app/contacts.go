package app

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type contactRequestResponse struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (s *Server) handleLookupUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		writeError(w, http.StatusBadRequest, "username_required")
		return
	}

	var found authUser
	err := s.db.QueryRowContext(r.Context(),
		"SELECT id, username FROM users WHERE username = ? AND id != ?",
		username,
		user.ID,
	).Scan(&found.ID, &found.Username)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed")
		return
	}
	writeJSON(w, http.StatusOK, found)
}

func (s *Server) handleCreateContactRequest(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		writeError(w, http.StatusBadRequest, "username_required")
		return
	}

	var recipient authUser
	err := s.db.QueryRowContext(r.Context(),
		"SELECT id, username FROM users WHERE username = ?",
		username,
	).Scan(&recipient.ID, &recipient.Username)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "user_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "contact_request_failed")
		return
	}
	if recipient.ID == user.ID {
		writeError(w, http.StatusBadRequest, "cannot_contact_self")
		return
	}
	if s.areContacts(r, user.ID, recipient.ID) {
		writeError(w, http.StatusConflict, "already_contacts")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO contact_requests (requester_id, recipient_id, status, created_at, updated_at)
		 VALUES (?, ?, 'pending', ?, ?)`,
		user.ID,
		recipient.ID,
		now,
		now,
	)
	if isUniqueViolation(err) {
		var existing contactRequestResponse
		err = s.db.QueryRowContext(r.Context(),
			`SELECT contact_requests.id, users.username, contact_requests.status, contact_requests.created_at
			 FROM contact_requests
			 JOIN users ON users.id = contact_requests.recipient_id
			 WHERE contact_requests.requester_id = ? AND contact_requests.recipient_id = ?`,
			user.ID,
			recipient.ID,
		).Scan(&existing.ID, &existing.Username, &existing.Status, &existing.CreatedAt)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "contact_request_failed")
			return
		}
		writeJSON(w, http.StatusOK, existing)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "contact_request_failed")
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "contact_request_failed")
		return
	}

	writeJSON(w, http.StatusCreated, contactRequestResponse{
		ID:        id,
		Username:  recipient.Username,
		Status:    "pending",
		CreatedAt: now,
	})
}

func (s *Server) handleIncomingContactRequests(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	rows, err := s.db.QueryContext(r.Context(),
		`SELECT contact_requests.id, users.username, contact_requests.status, contact_requests.created_at
		 FROM contact_requests
		 JOIN users ON users.id = contact_requests.requester_id
		 WHERE contact_requests.recipient_id = ? AND contact_requests.status = 'pending'
		 ORDER BY contact_requests.id`,
		user.ID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_requests_failed")
		return
	}
	defer rows.Close()

	var requests []contactRequestResponse
	for rows.Next() {
		var request contactRequestResponse
		if err := rows.Scan(&request.ID, &request.Username, &request.Status, &request.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "list_requests_failed")
			return
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "list_requests_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": requests})
}

func (s *Server) handleAcceptContactRequest(w http.ResponseWriter, r *http.Request) {
	s.updateContactRequest(w, r, "accepted")
}

func (s *Server) handleRejectContactRequest(w http.ResponseWriter, r *http.Request) {
	s.updateContactRequest(w, r, "rejected")
}

func (s *Server) updateContactRequest(w http.ResponseWriter, r *http.Request, status string) {
	user, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	requestID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_id")
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update_request_failed")
		return
	}
	defer tx.Rollback()

	var requesterID int64
	err = tx.QueryRowContext(r.Context(),
		`SELECT requester_id FROM contact_requests
		 WHERE id = ? AND recipient_id = ? AND status = 'pending'`,
		requestID,
		user.ID,
	).Scan(&requesterID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "request_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update_request_failed")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(r.Context(),
		"UPDATE contact_requests SET status = ?, updated_at = ? WHERE id = ?",
		status,
		now,
		requestID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "update_request_failed")
		return
	}

	if status == "accepted" {
		low, high := sortedPair(user.ID, requesterID)
		if _, err := tx.ExecContext(r.Context(),
			"INSERT OR IGNORE INTO contacts (user_low_id, user_high_id, created_at) VALUES (?, ?, ?)",
			low,
			high,
			now,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "update_request_failed")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "update_request_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListContacts(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireAuth(w, r)
	if !ok {
		return
	}

	rows, err := s.db.QueryContext(r.Context(),
		`SELECT users.id, users.username
		 FROM contacts
		 JOIN users ON users.id = CASE
		   WHEN contacts.user_low_id = ? THEN contacts.user_high_id
		   ELSE contacts.user_low_id
		 END
		 WHERE contacts.user_low_id = ? OR contacts.user_high_id = ?
		 ORDER BY users.username`,
		user.ID,
		user.ID,
		user.ID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_contacts_failed")
		return
	}
	defer rows.Close()

	var contacts []authUser
	for rows.Next() {
		var contact authUser
		if err := rows.Scan(&contact.ID, &contact.Username); err != nil {
			writeError(w, http.StatusInternalServerError, "list_contacts_failed")
			return
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "list_contacts_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) (authUser, bool) {
	user, ok := s.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return authUser{}, false
	}
	return user, true
}

func (s *Server) areContacts(r *http.Request, a, b int64) bool {
	low, high := sortedPair(a, b)
	var exists int
	err := s.db.QueryRowContext(r.Context(),
		"SELECT 1 FROM contacts WHERE user_low_id = ? AND user_high_id = ?",
		low,
		high,
	).Scan(&exists)
	return err == nil
}

func sortedPair(a, b int64) (int64, int64) {
	if a < b {
		return a, b
	}
	return b, a
}
