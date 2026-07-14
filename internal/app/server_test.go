package app

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistrationScreensPwnedPasswords(t *testing.T) {
	password := "known-breached-password"
	digest := fmt.Sprintf("%X", sha1.Sum([]byte(password)))
	var receivedPrefix string
	var padded string
	hibp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPrefix = strings.TrimPrefix(r.URL.Path, "/")
		padded = r.Header.Get("Add-Padding")
		fmt.Fprintf(w, "%s:42\n00000000000000000000000000000000000:0\n", digest[5:])
	}))
	defer hibp.Close()

	srv := newTestServerWithConfig(t, Config{
		Addr: "127.0.0.1:0", DatabasePath: filepath.Join(t.TempDir(), "private-direct.db"),
		OperatorToken: "operator-secret", JWTSecret: "test-jwt-secret",
		STUNServers: []string{"stun:test.example"}, PwnedPasswordsURL: hibp.URL,
	})
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	createInvite(t, httpSrv.URL, "invite-pwned")

	res := postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-pwned", "username": "alice", "password": password,
	})
	assertStatus(t, res, http.StatusBadRequest)
	assertErrorCode(t, res, "password_breached")
	if receivedPrefix != digest[:5] || padded != "true" {
		t.Fatalf("HIBP request prefix=%q padding=%q", receivedPrefix, padded)
	}
}

func TestRegistrationWarnsWhenPwnedPasswordCheckFails(t *testing.T) {
	hibp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	hibpURL := hibp.URL
	hibp.Close()

	srv := newTestServerWithConfig(t, Config{
		Addr: "127.0.0.1:0", DatabasePath: filepath.Join(t.TempDir(), "private-direct.db"),
		OperatorToken: "operator-secret", JWTSecret: "test-jwt-secret",
		STUNServers: []string{"stun:test.example"}, PwnedPasswordsURL: hibpURL,
	})
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	createInvite(t, httpSrv.URL, "invite-warning")

	res := postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-warning", "username": "alice", "password": "safe-password-value",
	})
	assertStatus(t, res, http.StatusCreated)
	var body map[string]any
	decodeResponse(t, res, &body)
	if body["warning"] != "password_breach_check_unavailable" {
		t.Fatalf("warning = %v", body["warning"])
	}
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res, err := http.Get(httpSrv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	return newTestServerWithConfig(t, Config{
		Addr:          "127.0.0.1:0",
		DatabasePath:  filepath.Join(t.TempDir(), "private-direct.db"),
		OperatorToken: "operator-secret",
		JWTSecret:     "test-jwt-secret",
		STUNServers:   []string{"stun:test.example"},
	})
}

func newTestServerWithConfig(t *testing.T, cfg Config) *Server {
	t.Helper()

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			t.Fatalf("close server: %v", err)
		}
	})
	return srv
}

func TestInviteRegistration(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	createInvite(t, httpSrv.URL, "invite-one")

	res := postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "  Alice  ",
		"password":    "secret-password",
	})
	assertStatus(t, res, http.StatusCreated)

	var body struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	decodeResponse(t, res, &body)
	if body.ID == 0 {
		t.Fatalf("user id = 0, want created id")
	}
	if body.Username != "alice" {
		t.Fatalf("username = %q, want alice", body.Username)
	}

	var storedHash string
	if err := srv.db.QueryRow("SELECT password_hash FROM users WHERE username = ?", "alice").Scan(&storedHash); err != nil {
		t.Fatalf("query password hash: %v", err)
	}
	if storedHash == "secret-password" {
		t.Fatalf("password stored as plaintext")
	}
}

func TestRegisterRejectsInvalidOrReusedInvite(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res := postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "missing",
		"username":    "alice",
		"password":    "secret-password",
	})
	assertStatus(t, res, http.StatusBadRequest)
	assertErrorCode(t, res, "invalid_invite")

	createInvite(t, httpSrv.URL, "invite-one")
	res = postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "alice",
		"password":    "secret-password",
	})
	assertStatus(t, res, http.StatusCreated)

	res = postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "bob",
		"password":    "secret-password",
	})
	assertStatus(t, res, http.StatusBadRequest)
	assertErrorCode(t, res, "invite_used")
}

func TestRegisterValidationBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantCode string
	}{
		{name: "username two characters", username: "ab", password: "secret-password", wantCode: "invalid_username"},
		{name: "username three characters", username: "a._", password: "secret-password"},
		{name: "username thirty two characters", username: strings.Repeat("a", 32), password: "secret-password"},
		{name: "username thirty three characters", username: strings.Repeat("a", 33), password: "secret-password", wantCode: "invalid_username"},
		{name: "username starts with punctuation", username: ".alice", password: "secret-password", wantCode: "invalid_username"},
		{name: "username contains at sign", username: "@alice", password: "secret-password", wantCode: "invalid_username"},
		{name: "username is non ascii", username: "alicé", password: "secret-password", wantCode: "invalid_username"},
		{name: "password eleven bytes", username: "alice", password: strings.Repeat("a", 11), wantCode: "invalid_password"},
		{name: "password twelve bytes", username: "alice", password: strings.Repeat("é", 6)},
		{name: "password seventy two bytes", username: "alice", password: strings.Repeat("é", 36)},
		{name: "password seventy three bytes", username: "alice", password: strings.Repeat("a", 73), wantCode: "invalid_password"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			httpSrv := httptest.NewServer(srv.Handler())
			defer httpSrv.Close()
			invite := fmt.Sprintf("invite-%d", i)
			createInvite(t, httpSrv.URL, invite)

			res := postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
				"invite_code": invite,
				"username":    tt.username,
				"password":    tt.password,
			})
			if tt.wantCode == "" {
				assertStatus(t, res, http.StatusCreated)
				res.Body.Close()
				return
			}
			assertStatus(t, res, http.StatusBadRequest)
			assertErrorCode(t, res, tt.wantCode)

			var usedBy *int64
			if err := srv.db.QueryRow("SELECT used_by_user_id FROM invites WHERE code = ?", invite).Scan(&usedBy); err != nil {
				t.Fatalf("query invite: %v", err)
			}
			if usedBy != nil {
				t.Fatalf("invalid registration consumed invite for user %d", *usedBy)
			}
		})
	}
}

func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	createInvite(t, httpSrv.URL, "invite-one")
	createInvite(t, httpSrv.URL, "invite-two")

	res := postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "alice",
		"password":    "secret-password",
	})
	assertStatus(t, res, http.StatusCreated)

	res = postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-two",
		"username":    "alice",
		"password":    "other-password",
	})
	assertStatus(t, res, http.StatusConflict)
	assertErrorCode(t, res, "username_exists")

	var usedBy *int64
	if err := srv.db.QueryRow("SELECT used_by_user_id FROM invites WHERE code = ?", "invite-two").Scan(&usedBy); err != nil {
		t.Fatalf("query invite: %v", err)
	}
	if usedBy != nil {
		t.Fatalf("duplicate username consumed invite for user %d", *usedBy)
	}

	res = postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{
		"invite_code": "invite-two",
		"username":    "bob",
		"password":    "other-password",
	})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
}

func TestRegisterRequiresEveryField(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]string
		wantCode string
	}{
		{
			name:     "invite code",
			body:     map[string]string{"username": "alice", "password": "secret-password"},
			wantCode: "invite_code_required",
		},
		{
			name:     "username",
			body:     map[string]string{"invite_code": "invite-one", "password": "secret-password"},
			wantCode: "username_required",
		},
		{
			name:     "password",
			body:     map[string]string{"invite_code": "invite-one", "username": "alice"},
			wantCode: "password_required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			httpSrv := httptest.NewServer(srv.Handler())
			defer httpSrv.Close()
			createInvite(t, httpSrv.URL, "invite-one")

			res := postJSON(t, httpSrv.URL+"/api/register", nil, tt.body)
			assertStatus(t, res, http.StatusBadRequest)
			assertErrorCode(t, res, tt.wantCode)
		})
	}
}

func TestCreateInviteRequiresOperatorToken(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res := postJSON(t, httpSrv.URL+"/api/operator/invites", nil, map[string]string{"code": "invite-one"})
	assertStatus(t, res, http.StatusUnauthorized)
	res.Body.Close()
}

func TestSessionLifecycle(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-one", "alice", "secret-password")

	loginRes := postJSON(t, httpSrv.URL+"/api/login", nil, map[string]string{
		"username": "alice",
		"password": "secret-password",
	})
	assertStatus(t, loginRes, http.StatusOK)
	refresh := findCookie(t, loginRes, refreshCookie)
	if !refresh.HttpOnly {
		t.Fatalf("refresh cookie HttpOnly = false, want true")
	}
	var loginBody struct {
		AccessToken string   `json:"access_token"`
		TokenType   string   `json:"token_type"`
		User        authUser `json:"user"`
	}
	decodeResponse(t, loginRes, &loginBody)
	if loginBody.AccessToken == "" {
		t.Fatalf("access token is empty")
	}
	if loginBody.TokenType != "Bearer" {
		t.Fatalf("token type = %q, want Bearer", loginBody.TokenType)
	}
	if loginBody.User.Username != "alice" || loginBody.User.ID == 0 {
		t.Fatalf("login user = %+v, want alice with id", loginBody.User)
	}

	refreshRes := postJSON(t, httpSrv.URL+"/api/refresh", map[string]string{
		"Cookie": refresh.String(),
	}, map[string]string{})
	assertStatus(t, refreshRes, http.StatusOK)
	var refreshBody struct {
		AccessToken string   `json:"access_token"`
		User        authUser `json:"user"`
	}
	decodeResponse(t, refreshRes, &refreshBody)
	if refreshBody.AccessToken == "" {
		t.Fatalf("refreshed access token is empty")
	}
	if refreshBody.User.Username != "alice" {
		t.Fatalf("refresh user = %+v, want alice", refreshBody.User)
	}

	logoutRes := postJSON(t, httpSrv.URL+"/api/logout", map[string]string{
		"Cookie": refresh.String(),
	}, map[string]string{})
	assertStatus(t, logoutRes, http.StatusNoContent)
	logoutRes.Body.Close()

	refreshRes = postJSON(t, httpSrv.URL+"/api/refresh", map[string]string{
		"Cookie": refresh.String(),
	}, map[string]string{})
	assertStatus(t, refreshRes, http.StatusUnauthorized)
	refreshRes.Body.Close()
}

func TestLoginRejectsBadPassword(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-one", "alice", "secret-password")

	res := postJSON(t, httpSrv.URL+"/api/login", nil, map[string]string{
		"username": "alice",
		"password": "wrong",
	})
	assertStatus(t, res, http.StatusUnauthorized)
	res.Body.Close()
}

func TestRefreshRequiresCookie(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res := postJSON(t, httpSrv.URL+"/api/refresh", nil, map[string]string{})
	assertStatus(t, res, http.StatusUnauthorized)
	res.Body.Close()
}

func createInvite(t *testing.T, baseURL, code string) {
	t.Helper()
	res := postJSON(t, baseURL+"/api/operator/invites", map[string]string{
		"X-Operator-Token": "operator-secret",
	}, map[string]string{"code": code})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
}

func registerUser(t *testing.T, baseURL, inviteCode, username, password string) {
	t.Helper()
	createInvite(t, baseURL, inviteCode)
	res := postJSON(t, baseURL+"/api/register", nil, map[string]string{
		"invite_code": inviteCode,
		"username":    username,
		"password":    password,
	})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
}

func findCookie(t *testing.T, res *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range res.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func postJSON(t *testing.T, url string, headers map[string]string, body any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return res
}

func assertStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want %d, body = %s", res.StatusCode, want, string(body))
	}
}

func assertErrorCode(t *testing.T, res *http.Response, want string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	decodeResponse(t, res, &body)
	if body.Error != want {
		t.Fatalf("error = %q, want %q", body.Error, want)
	}
}

func decodeResponse(t *testing.T, res *http.Response, dst any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
