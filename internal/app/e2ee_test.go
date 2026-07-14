package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestE2EESetupRegistersFirstDeviceAndMarksAccountReady(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")

	res := postJSON(t, httpSrv.URL+"/api/e2ee/setup", map[string]string{
		"Authorization": "Bearer " + token,
	}, map[string]any{
		"device_id": "device-one", "identity_keys": map[string]any{"master_key": "public"},
		"device_keys":        map[string]any{"device_keys": map[string]string{"ed25519:device-one": "public"}},
		"wrapped_master_key": "ciphertext", "kdf_salt": "salt", "protocol_version": 1,
	})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()

	var accountCount, deviceCount int
	if err := srv.db.QueryRow("SELECT COUNT(*) FROM e2ee_accounts WHERE user_id = 1").Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if err := srv.db.QueryRow("SELECT COUNT(*) FROM e2ee_devices WHERE user_id = 1").Scan(&deviceCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 1 || deviceCount != 1 {
		t.Fatalf("accounts=%d devices=%d", accountCount, deviceCount)
	}

	loginRes := postJSON(t, httpSrv.URL+"/api/login", nil, map[string]string{
		"username": "alice", "password": "secret-password",
	})
	assertStatus(t, loginRes, http.StatusOK)
	var loginBody struct {
		User authUser `json:"user"`
	}
	decodeResponse(t, loginRes, &loginBody)
	if !loginBody.User.E2EEReady {
		t.Fatal("e2ee_ready=false after setup")
	}
}

func TestE2EESetupRejectsSecretsAndDuplicateSetup(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")
	headers := map[string]string{"Authorization": "Bearer " + token}
	payload := map[string]any{
		"device_id": "device-one", "identity_keys": json.RawMessage(`{"master_key":"public"}`),
		"device_keys":        json.RawMessage(`{"device_keys":{"ed25519:device-one":"public"}}`),
		"wrapped_master_key": "ciphertext", "kdf_salt": "salt", "protocol_version": 1,
	}
	res := postJSON(t, httpSrv.URL+"/api/e2ee/setup", headers, payload)
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	res = postJSON(t, httpSrv.URL+"/api/e2ee/setup", headers, payload)
	assertStatus(t, res, http.StatusConflict)
	assertErrorCode(t, res, "e2ee_already_setup")
}
