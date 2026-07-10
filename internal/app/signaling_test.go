package app

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebRTCSignalingForwarding(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-pass")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-pass")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-pass")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-pass")
	bobID := lookupUserID(t, httpSrv.URL, aliceToken, "bob")
	aliceID := lookupUserID(t, httpSrv.URL, bobToken, "alice")
	createAcceptedContact(t, httpSrv.URL, aliceToken, bobToken, "bob")

	aliceWS := dialWS(t, httpSrv.URL, aliceToken)
	defer aliceWS.Close()
	bobWS := dialWS(t, httpSrv.URL, bobToken)
	defer bobWS.Close()
	readPresence(t, aliceWS)

	writeWS(t, aliceWS, map[string]any{
		"type":        "signal",
		"to_user_id":  bobID,
		"signal_type": "offer",
		"payload":     map[string]string{"sdp": "offer-sdp"},
	})
	offer := readSignal(t, bobWS)
	if offer.From.Username != "alice" || offer.SignalType != "offer" || string(offer.Payload) != `{"sdp":"offer-sdp"}` {
		t.Fatalf("offer = %+v, want alice offer", offer)
	}

	writeWS(t, bobWS, map[string]any{
		"type":        "signal",
		"to_user_id":  aliceID,
		"signal_type": "answer",
		"payload":     map[string]string{"sdp": "answer-sdp"},
	})
	answer := readSignal(t, aliceWS)
	if answer.From.Username != "bob" || answer.SignalType != "answer" || string(answer.Payload) != `{"sdp":"answer-sdp"}` {
		t.Fatalf("answer = %+v, want bob answer", answer)
	}

	writeWS(t, bobWS, map[string]any{
		"type":        "signal",
		"to_user_id":  aliceID,
		"signal_type": "ice",
		"payload":     map[string]string{"candidate": "candidate-1"},
	})
	ice := readSignal(t, aliceWS)
	if ice.From.Username != "bob" || ice.SignalType != "ice" || string(ice.Payload) != `{"candidate":"candidate-1"}` {
		t.Fatalf("ice = %+v, want bob ice", ice)
	}

	var signalTables int
	if err := srv.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name LIKE '%signal%'").Scan(&signalTables); err != nil {
		t.Fatalf("query sqlite schema: %v", err)
	}
	if signalTables != 0 {
		t.Fatalf("signal tables = %d, want 0", signalTables)
	}
}

func TestWebRTCSignalingRejectsNonContact(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-pass")
	registerUser(t, httpSrv.URL, "invite-charlie", "charlie", "secret-pass")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-pass")
	charlieToken := loginUser(t, httpSrv.URL, "charlie", "secret-pass")
	charlieID := lookupUserID(t, httpSrv.URL, aliceToken, "charlie")

	aliceWS := dialWS(t, httpSrv.URL, aliceToken)
	defer aliceWS.Close()
	charlieWS := dialWS(t, httpSrv.URL, charlieToken)
	defer charlieWS.Close()

	writeWS(t, aliceWS, map[string]any{
		"type":        "signal",
		"to_user_id":  charlieID,
		"signal_type": "offer",
		"payload":     map[string]string{"sdp": "offer-sdp"},
	})
	errEvent := readWSError(t, aliceWS)
	if errEvent.Error != "not_contact" {
		t.Fatalf("error = %q, want not_contact", errEvent.Error)
	}
	assertNoPresence(t, charlieWS)
}

func TestWebRTCSignalingRejectsOfflineContact(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-pass")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-pass")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-pass")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-pass")
	bobID := lookupUserID(t, httpSrv.URL, aliceToken, "bob")
	createAcceptedContact(t, httpSrv.URL, aliceToken, bobToken, "bob")

	aliceWS := dialWS(t, httpSrv.URL, aliceToken)
	defer aliceWS.Close()

	writeWS(t, aliceWS, map[string]any{
		"type":        "signal",
		"to_user_id":  bobID,
		"signal_type": "offer",
		"payload":     map[string]string{"sdp": "offer-sdp"},
	})
	errEvent := readWSError(t, aliceWS)
	if errEvent.Error != "target_offline" {
		t.Fatalf("error = %q, want target_offline", errEvent.Error)
	}
}

func TestWebSocketRejectsServerRelayedChat(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-pass")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-pass")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-pass")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-pass")
	bobID := lookupUserID(t, httpSrv.URL, aliceToken, "bob")
	createAcceptedContact(t, httpSrv.URL, aliceToken, bobToken, "bob")

	aliceWS := dialWS(t, httpSrv.URL, aliceToken)
	defer aliceWS.Close()
	bobWS := dialWS(t, httpSrv.URL, bobToken)
	defer bobWS.Close()
	readPresence(t, aliceWS)

	writeWS(t, aliceWS, map[string]any{
		"type":       "chat",
		"to_user_id": bobID,
		"payload":    map[string]string{"text": "hello"},
	})
	errEvent := readWSError(t, aliceWS)
	if errEvent.Error != "unsupported_message_type" {
		t.Fatalf("error = %q, want unsupported_message_type", errEvent.Error)
	}
	assertNoPresence(t, bobWS)
}

func lookupUserID(t *testing.T, baseURL, token, username string) int64 {
	t.Helper()
	res := getJSON(t, baseURL+"/users/lookup?username="+url.QueryEscape(username), bearerHeaders(token))
	assertStatus(t, res, 200)
	var user authUser
	decodeResponse(t, res, &user)
	return user.ID
}

func writeWS(t *testing.T, conn *websocket.Conn, message any) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err := conn.WriteJSON(message); err != nil {
		t.Fatalf("write websocket: %v", err)
	}
}

func readSignal(t *testing.T, conn *websocket.Conn) signalEvent {
	t.Helper()
	for i := 0; i < 3; i++ {
		messageType, data := readRawWSEvent(t, conn)
		if messageType != "signal" {
			continue
		}
		var event signalEvent
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatalf("decode signal: %v", err)
		}
		return event
	}
	t.Fatalf("signal event not received")
	return signalEvent{}
}

func readWSError(t *testing.T, conn *websocket.Conn) wsErrorEvent {
	t.Helper()
	for i := 0; i < 3; i++ {
		messageType, data := readRawWSEvent(t, conn)
		if messageType != "error" {
			continue
		}
		var event wsErrorEvent
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatalf("decode websocket error: %v", err)
		}
		return event
	}
	t.Fatalf("error event not received")
	return wsErrorEvent{}
}

func readRawWSEvent(t *testing.T, conn *websocket.Conn) (string, []byte) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket: %v", err)
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode websocket envelope: %v", err)
	}
	return envelope.Type, data
}
