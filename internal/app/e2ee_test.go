package app

import (
	"bytes"
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

func TestE2EERecoveryRegistersDeviceAndStoresOpaqueBackup(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")
	headers := map[string]string{"Authorization": "Bearer " + token}
	setup := map[string]any{"device_id": "one", "identity_keys": map[string]string{"master": "public"}, "device_keys": map[string]string{"key": "public"}, "wrapped_master_key": "wrapped", "kdf_salt": "salt", "protocol_version": 1}
	res := postJSON(t, httpSrv.URL+"/api/e2ee/setup", headers, setup)
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()

	data, _ := json.Marshal(map[string]string{"ciphertext": "opaque-room-keys"})
	req, _ := http.NewRequest(http.MethodPut, httpSrv.URL+"/api/e2ee/key-backup", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ = http.DefaultClient.Do(req)
	assertStatus(t, res, http.StatusNoContent)
	res.Body.Close()
	res = postJSON(t, httpSrv.URL+"/api/e2ee/recovery/devices", headers, map[string]any{"device_id": "two", "device_keys": map[string]string{"key": "new-public"}})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	res = getJSON(t, httpSrv.URL+"/api/e2ee/recovery", headers)
	assertStatus(t, res, http.StatusOK)
	var body struct {
		WrappedMasterKey string `json:"wrapped_master_key"`
		KDFSalt          string `json:"kdf_salt"`
		KeyBackup        string `json:"key_backup"`
	}
	decodeResponse(t, res, &body)
	if body.WrappedMasterKey != "wrapped" || body.KDFSalt != "salt" || body.KeyBackup != "opaque-room-keys" {
		t.Fatalf("unexpected recovery response: %+v", body)
	}

	var devices int
	if err := srv.db.QueryRow(`SELECT COUNT(*) FROM e2ee_devices WHERE user_id=1`).Scan(&devices); err != nil || devices != 2 {
		t.Fatalf("devices=%d err=%v", devices, err)
	}
}

func TestE2EEDevicesCanBeListedAndRevoked(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")
	headers := bearerHeaders(token)
	res := postJSON(t, httpSrv.URL+"/api/e2ee/setup", headers, map[string]any{"device_id": "one", "device_name": "Alice laptop", "identity_keys": map[string]string{"master": "public"}, "device_keys": map[string]string{"key": "public"}, "wrapped_master_key": "wrapped", "kdf_salt": "salt", "protocol_version": 1})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	res = getJSON(t, httpSrv.URL+"/api/e2ee/devices", headers)
	assertStatus(t, res, http.StatusOK)
	var listed struct {
		Devices []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"devices"`
		Limit int `json:"limit"`
	}
	decodeResponse(t, res, &listed)
	if len(listed.Devices) != 1 || listed.Devices[0].Name != "Alice laptop" || listed.Limit != 10 {
		t.Fatalf("unexpected devices: %+v", listed)
	}
	req, _ := http.NewRequest(http.MethodDelete, httpSrv.URL+"/api/e2ee/devices/one", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ = http.DefaultClient.Do(req)
	assertStatus(t, res, http.StatusNoContent)
	res.Body.Close()
	res = getJSON(t, httpSrv.URL+"/api/e2ee/sync?device_id=one", headers)
	assertStatus(t, res, http.StatusForbidden)
	assertErrorCode(t, res, "device_revoked")
}
