package app

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	refreshCookie   = "refresh_token"
)

type authUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	username := normalizeUsername(req.Username)
	if !validUsername(username) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	var user authUser
	var passwordHash string
	err := s.db.QueryRowContext(r.Context(),
		"SELECT id, username, password_hash FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &passwordHash)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login_failed")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	if err := s.issueSession(w, r, user); err != nil {
		writeError(w, http.StatusInternalServerError, "login_failed")
		return
	}
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookie)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "refresh_required")
		return
	}

	user, err := s.userForRefreshToken(r, cookie.Value)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusUnauthorized, "invalid_refresh")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refresh_failed")
		return
	}

	accessToken, err := s.signAccessToken(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "refresh_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(accessTokenTTL.Seconds()),
		"user":         user,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookie)
	if err == nil && cookie.Value != "" {
		_, _ = s.db.ExecContext(r.Context(),
			"UPDATE refresh_sessions SET revoked_at = ? WHERE token_hash = ?",
			time.Now().UTC().Format(time.RFC3339),
			hashToken(cookie.Value),
		)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request, user authUser) error {
	refreshToken, err := newToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(r.Context(),
		`INSERT INTO refresh_sessions (user_id, token_hash, expires_at, created_at)
		 VALUES (?, ?, ?, ?)`,
		user.ID,
		hashToken(refreshToken),
		now.Add(refreshTokenTTL).Format(time.RFC3339),
		now.Format(time.RFC3339),
	); err != nil {
		return err
	}

	accessToken, err := s.signAccessToken(user)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    refreshToken,
		Path:     "/",
		Expires:  now.Add(refreshTokenTTL),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(accessTokenTTL.Seconds()),
		"user":         user,
	})
	return nil
}

func (s *Server) userForRefreshToken(r *http.Request, token string) (authUser, error) {
	var user authUser
	var expiresAt string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT users.id, users.username, refresh_sessions.expires_at
		 FROM refresh_sessions
		 JOIN users ON users.id = refresh_sessions.user_id
		 WHERE refresh_sessions.token_hash = ?
		   AND refresh_sessions.revoked_at IS NULL`,
		hashToken(token),
	).Scan(&user.ID, &user.Username, &expiresAt)
	if err != nil {
		return authUser{}, err
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return authUser{}, err
	}
	if time.Now().UTC().After(expiry) {
		return authUser{}, sql.ErrNoRows
	}
	return user, nil
}

func (s *Server) signAccessToken(user authUser) (string, error) {
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      strconv.FormatInt(user.ID, 10),
		"username": user.Username,
		"iat":      now.Unix(),
		"exp":      now.Add(accessTokenTTL).Unix(),
	})
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *Server) authenticate(r *http.Request) (authUser, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return authUser{}, false
	}
	return s.authenticateToken(strings.TrimPrefix(header, "Bearer "))
}

func (s *Server) authenticateToken(raw string) (authUser, bool) {
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return authUser{}, false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return authUser{}, false
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return authUser{}, false
	}
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return authUser{}, false
	}
	username, _ := claims["username"].(string)
	if username == "" {
		return authUser{}, false
	}
	return authUser{ID: userID, Username: username}, true
}

func newToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}
