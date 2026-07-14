package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestEncryptedMessagePersistenceIsIdempotentAndOrdered(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	if _, err := srv.db.Exec("INSERT INTO contacts (user_low_id,user_high_id,created_at) VALUES (1,2,'now')"); err != nil {
		t.Fatal(err)
	}
	alice := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bob := loginUser(t, httpSrv.URL, "bob", "secret-password")
	payload := map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "to": 2, "ciphertext": map[string]any{"algorithm": "m.megolm.v1.aes-sha2", "ciphertext": "opaque"}}

	res := postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(alice), payload)
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	res = postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(alice), payload)
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	var count int
	var stored string
	if err := srv.db.QueryRow("SELECT COUNT(*),ciphertext FROM encrypted_messages").Scan(&count, &stored); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}
	if stored == "opaque" {
		t.Fatal("stored plaintext instead of envelope")
	}

	res = getJSON(t, httpSrv.URL+"/api/messages?contact_id=1&limit=50", bearerHeaders(bob))
	assertStatus(t, res, http.StatusOK)
	var body struct {
		Messages []struct {
			ID         string         `json:"id"`
			Sequence   int64          `json:"sequence"`
			Ciphertext map[string]any `json:"ciphertext"`
		} `json:"messages"`
	}
	decodeResponse(t, res, &body)
	if len(body.Messages) != 1 || body.Messages[0].ID != payload["id"] || body.Messages[0].Sequence == 0 {
		t.Fatalf("messages=%+v", body.Messages)
	}
}

func TestSignedEventChainRejectsGapAlterationAndRegression(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "ia", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "ib", "bob", "secret-password")
	_, _ = srv.db.Exec("INSERT INTO contacts(user_low_id,user_high_id,created_at) VALUES(1,2,'now')")
	_, _ = srv.db.Exec(`INSERT INTO e2ee_accounts(user_id,protocol_version,identity_keys,wrapped_master_key,kdf_salt,created_at) VALUES(1,1,'{}','wrapped','salt','now')`)
	_, _ = srv.db.Exec(`INSERT INTO e2ee_accounts(user_id,protocol_version,identity_keys,wrapped_master_key,kdf_salt,created_at) VALUES(2,1,'{}','wrapped','salt','now')`)
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	keys, _ := json.Marshal(map[string]any{"keys": map[string]string{"ed25519:device": base64.RawStdEncoding.EncodeToString(public)}})
	_, _ = srv.db.Exec(`INSERT INTO e2ee_devices(id,user_id,public_keys,created_at,last_seen_at) VALUES('device',1,?,'now','now')`, string(keys))
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")
	cipher := map[string]string{"body": "opaque"}
	makePayload := func(id string, index int64, previous string, alter bool) map[string]any {
		raw, _ := json.Marshal(cipher)
		sum := sha256.Sum256([]byte(id + "|2|" + string(raw) + "|" + strconv.FormatInt(index, 10) + "|" + previous))
		hash := base64.RawStdEncoding.EncodeToString(sum[:])
		if alter {
			hash = "altered"
		}
		return map[string]any{"id": id, "to": 2, "ciphertext": cipher, "chain": map[string]any{"device_id": "device", "index": index, "previous_hash": previous, "event_hash": hash, "signature": base64.RawStdEncoding.EncodeToString(ed25519.Sign(private, []byte(hash)))}}
	}
	first := makePayload("550e8400-e29b-41d4-a716-446655440000", 1, "", false)
	res := postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(token), first)
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	firstHash := first["chain"].(map[string]any)["event_hash"].(string)
	for _, payload := range []map[string]any{makePayload("550e8400-e29b-41d4-a716-446655440001", 3, firstHash, false), makePayload("550e8400-e29b-41d4-a716-446655440002", 2, firstHash, true), makePayload("550e8400-e29b-41d4-a716-446655440003", 1, "", false)} {
		res = postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(token), payload)
		assertStatus(t, res, http.StatusConflict)
		assertErrorCode(t, res, "invalid_event_chain")
	}
	var count int
	_ = srv.db.QueryRow(`SELECT COUNT(*) FROM encrypted_messages`).Scan(&count)
	if count != 1 {
		t.Fatalf("messages=%d", count)
	}
}

func TestUnreadCursorSynchronizesWithoutContactReceipt(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "ia", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "ib", "bob", "secret-password")
	_, _ = srv.db.Exec("INSERT INTO contacts(user_low_id,user_high_id,created_at) VALUES(1,2,'now')")
	alice := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bob := loginUser(t, httpSrv.URL, "bob", "secret-password")
	res := postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(alice), map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "to": 2, "ciphertext": map[string]string{"body": "opaque"}})
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	res = getJSON(t, httpSrv.URL+"/api/messages/unread", bearerHeaders(bob))
	assertStatus(t, res, http.StatusOK)
	var before struct {
		Unread map[string]int `json:"unread"`
	}
	decodeResponse(t, res, &before)
	if before.Unread["1"] != 1 {
		t.Fatalf("unread=%v", before.Unread)
	}
	data, _ := json.Marshal(map[string]int{"sequence": 1})
	req, _ := http.NewRequest(http.MethodPut, httpSrv.URL+"/api/conversations/1/read", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bob)
	res, _ = http.DefaultClient.Do(req)
	assertStatus(t, res, http.StatusNoContent)
	res.Body.Close()
	res = getJSON(t, httpSrv.URL+"/api/messages/unread", bearerHeaders(bob))
	assertStatus(t, res, http.StatusOK)
	var after struct {
		Unread map[string]int `json:"unread"`
	}
	decodeResponse(t, res, &after)
	if after.Unread["1"] != 0 {
		t.Fatalf("unread after read=%v", after.Unread)
	}
	var deliveries int
	_ = srv.db.QueryRow("SELECT COUNT(*) FROM message_deliveries").Scan(&deliveries)
	if deliveries != 0 {
		t.Fatalf("read cursor created contact-visible receipt")
	}
}

func TestEncryptedHistoryPaginatesByCanonicalSequence(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "ia", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "ib", "bob", "secret-password")
	_, _ = srv.db.Exec("INSERT INTO contacts(user_low_id,user_high_id,created_at) VALUES(1,2,'now')")
	for i := 1; i <= 51; i++ {
		_, err := srv.db.Exec(`INSERT INTO encrypted_messages(message_id,sender_id,recipient_id,ciphertext,created_at) VALUES(?,?,?,?,?)`, fmt.Sprintf("00000000-0000-4000-8000-%012d", i), 1, 2, `{"body":"opaque"}`, fmt.Sprintf("time-%d", 52-i))
		if err != nil {
			t.Fatal(err)
		}
	}
	token := loginUser(t, httpSrv.URL, "bob", "secret-password")
	res := getJSON(t, httpSrv.URL+"/api/messages?contact_id=1&limit=50", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var first struct {
		Messages []struct {
			ID       string `json:"id"`
			Sequence int64  `json:"sequence"`
		} `json:"messages"`
	}
	decodeResponse(t, res, &first)
	if len(first.Messages) != 50 || first.Messages[0].Sequence != 51 || first.Messages[49].Sequence != 2 {
		t.Fatalf("first page bounds=%+v", first.Messages)
	}
	res = getJSON(t, httpSrv.URL+"/api/messages?contact_id=1&limit=50&before=2", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var second struct {
		Messages []struct {
			Sequence int64 `json:"sequence"`
		} `json:"messages"`
	}
	decodeResponse(t, res, &second)
	if len(second.Messages) != 1 || second.Messages[0].Sequence != 1 {
		t.Fatalf("second page=%+v", second.Messages)
	}
}

func TestSignedTombstoneDeletesForBothAndPreventsRestore(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "ia", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "ib", "bob", "secret-password")
	_, _ = srv.db.Exec("INSERT INTO contacts(user_low_id,user_high_id,created_at) VALUES(1,2,'now')")
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	keys, _ := json.Marshal(map[string]any{"keys": map[string]string{"ed25519:alice-device": base64.RawStdEncoding.EncodeToString(public)}})
	_, _ = srv.db.Exec(`INSERT INTO e2ee_devices(id,user_id,public_keys,created_at,last_seen_at) VALUES('alice-device',1,?,'now','now')`, string(keys))
	alice := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bob := loginUser(t, httpSrv.URL, "bob", "secret-password")
	id := "550e8400-e29b-41d4-a716-446655440000"
	payload := map[string]any{"id": id, "to": 2, "ciphertext": map[string]string{"body": "opaque"}}
	res := postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(alice), payload)
	assertStatus(t, res, http.StatusCreated)
	res.Body.Close()
	created := "2026-07-14T12:00:00Z"
	signature := base64.RawStdEncoding.EncodeToString(ed25519.Sign(private, []byte(id+"|both|"+created)))
	data, _ := json.Marshal(map[string]string{"scope": "both", "device_id": "alice-device", "created_at": created, "signature": signature})
	req, _ := http.NewRequest(http.MethodDelete, httpSrv.URL+"/api/messages/"+id, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+alice)
	res, _ = http.DefaultClient.Do(req)
	assertStatus(t, res, http.StatusNoContent)
	res.Body.Close()
	res = postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(alice), payload)
	assertStatus(t, res, http.StatusGone)
	assertErrorCode(t, res, "message_deleted")
	res = getJSON(t, httpSrv.URL+"/api/messages?contact_id=1", bearerHeaders(bob))
	assertStatus(t, res, http.StatusOK)
	var result struct {
		Messages []any    `json:"messages"`
		Deleted  []string `json:"deleted"`
	}
	decodeResponse(t, res, &result)
	if len(result.Messages) != 0 || len(result.Deleted) != 1 || result.Deleted[0] != id {
		t.Fatalf("result=%+v", result)
	}
}

func TestEncryptedMessageRejectsNonContactAndOversizeEnvelope(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	alice := loginUser(t, httpSrv.URL, "alice", "secret-password")
	res := postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(alice), map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "to": 2, "ciphertext": map[string]string{"body": "opaque"}})
	assertStatus(t, res, http.StatusForbidden)
	res.Body.Close()
}

func TestEncryptedMessageEnforcesQuotaAndRateLimit(t *testing.T) {
	tests := []struct {
		name  string
		quota int64
		burst int
		want  int
	}{
		{name: "quota", quota: 35, burst: 30, want: http.StatusInsufficientStorage},
		{name: "rate", quota: 1024, burst: 1, want: http.StatusTooManyRequests},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServerWithConfig(t, Config{Addr: "127.0.0.1:0", DatabasePath: t.TempDir() + "/db.sqlite", OperatorToken: "operator-secret", JWTSecret: "secret", STUNServers: []string{"stun:test"}, MessageQuotaBytes: tt.quota, MessageRatePerMinute: 1, MessageRateBurst: tt.burst})
			httpSrv := httptest.NewServer(srv.Handler())
			defer httpSrv.Close()
			registerUser(t, httpSrv.URL, "ia", "alice", "secret-password")
			registerUser(t, httpSrv.URL, "ib", "bob", "secret-password")
			_, _ = srv.db.Exec("INSERT INTO contacts(user_low_id,user_high_id,created_at) VALUES(1,2,'now')")
			token := loginUser(t, httpSrv.URL, "alice", "secret-password")
			payload := map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000", "to": 2, "ciphertext": map[string]string{"body": "1234567890"}}
			res := postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(token), payload)
			assertStatus(t, res, http.StatusCreated)
			res.Body.Close()
			payload["id"] = "550e8400-e29b-41d4-a716-446655440001"
			res = postJSON(t, httpSrv.URL+"/api/messages", bearerHeaders(token), payload)
			assertStatus(t, res, tt.want)
			res.Body.Close()
		})
	}
}
