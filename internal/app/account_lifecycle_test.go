package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOperatorResetDoesNotResetE2EEAndAccountDeletionReservesUsername(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "ia", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")
	res := postJSON(t, httpSrv.URL+"/api/e2ee/setup", bearerHeaders(token), map[string]any{"device_id": "one", "identity_keys": map[string]string{"master": "public"}, "device_keys": map[string]string{"key": "public"}, "wrapped_master_key": "wrapped", "kdf_salt": "salt", "protocol_version": 1})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	resetData, _ := json.Marshal(map[string]string{"password": "new-secret-password"})
	resetReq, _ := http.NewRequest(http.MethodPut, httpSrv.URL+"/api/operator/users/alice/password", bytes.NewReader(resetData))
	resetReq.Header.Set("Content-Type", "application/json")
	resetReq.Header.Set("X-Operator-Token", "operator-secret")
	res, _ = http.DefaultClient.Do(resetReq)
	assertStatus(t, res, http.StatusOK)
	var reset map[string]any
	decodeResponse(t, res, &reset)
	if reset["message_recovery"] != "authorized_device_or_recovery_phrase_required" {
		t.Fatalf("reset=%v", reset)
	}
	res = postJSON(t, httpSrv.URL+"/api/login", nil, map[string]string{"username": "alice", "password": "secret-password"})
	assertStatus(t, res, http.StatusUnauthorized)
	res.Body.Close()
	token = loginUser(t, httpSrv.URL, "alice", "new-secret-password")
	var accounts int
	_ = srv.db.QueryRow(`SELECT COUNT(*) FROM e2ee_accounts`).Scan(&accounts)
	if accounts != 1 {
		t.Fatal("password reset removed E2EE account")
	}
	data, _ := json.Marshal(map[string]string{"password": "new-secret-password"})
	req, _ := http.NewRequest(http.MethodDelete, httpSrv.URL+"/api/account", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ = http.DefaultClient.Do(req)
	assertStatus(t, res, http.StatusNoContent)
	res.Body.Close()
	var users, devices int
	_ = srv.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	_ = srv.db.QueryRow(`SELECT COUNT(*) FROM e2ee_devices`).Scan(&devices)
	if users != 0 || devices != 0 {
		t.Fatalf("users=%d devices=%d", users, devices)
	}
	createInvite(t, httpSrv.URL, "ia2")
	res = postJSON(t, httpSrv.URL+"/api/register", nil, map[string]string{"invite_code": "ia2", "username": "alice", "password": "another-secret-password"})
	assertStatus(t, res, http.StatusConflict)
	assertErrorCode(t, res, "username_reserved")
}
