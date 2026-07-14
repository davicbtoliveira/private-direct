package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
