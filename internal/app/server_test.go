package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

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

	srv, err := NewServer(Config{
		Addr:          "127.0.0.1:0",
		DatabasePath:  filepath.Join(t.TempDir(), "private-direct.db"),
		OperatorToken: "operator-secret",
	})
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

	res := postJSON(t, httpSrv.URL+"/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "alice",
		"password":    "secret-pass",
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
	if storedHash == "secret-pass" {
		t.Fatalf("password stored as plaintext")
	}
}

func TestRegisterRejectsInvalidOrReusedInvite(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res := postJSON(t, httpSrv.URL+"/register", nil, map[string]string{
		"invite_code": "missing",
		"username":    "alice",
		"password":    "secret-pass",
	})
	assertStatus(t, res, http.StatusBadRequest)

	createInvite(t, httpSrv.URL, "invite-one")
	res = postJSON(t, httpSrv.URL+"/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "alice",
		"password":    "secret-pass",
	})
	assertStatus(t, res, http.StatusCreated)

	res = postJSON(t, httpSrv.URL+"/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "bob",
		"password":    "secret-pass",
	})
	assertStatus(t, res, http.StatusBadRequest)
}

func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	createInvite(t, httpSrv.URL, "invite-one")
	createInvite(t, httpSrv.URL, "invite-two")

	res := postJSON(t, httpSrv.URL+"/register", nil, map[string]string{
		"invite_code": "invite-one",
		"username":    "alice",
		"password":    "secret-pass",
	})
	assertStatus(t, res, http.StatusCreated)

	res = postJSON(t, httpSrv.URL+"/register", nil, map[string]string{
		"invite_code": "invite-two",
		"username":    "alice",
		"password":    "other-pass",
	})
	assertStatus(t, res, http.StatusConflict)
}

func TestCreateInviteRequiresOperatorToken(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res := postJSON(t, httpSrv.URL+"/operator/invites", nil, map[string]string{"code": "invite-one"})
	assertStatus(t, res, http.StatusUnauthorized)
}

func createInvite(t *testing.T, baseURL, code string) {
	t.Helper()
	res := postJSON(t, baseURL+"/operator/invites", map[string]string{
		"X-Operator-Token": "operator-secret",
	}, map[string]string{"code": code})
	assertStatus(t, res, http.StatusCreated)
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

func decodeResponse(t *testing.T, res *http.Response, dst any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
