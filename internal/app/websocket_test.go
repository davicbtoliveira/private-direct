package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketRequiresAccessToken(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	_, res, err := websocket.DefaultDialer.Dial(wsURL(httpSrv.URL)+"/api/ws", nil)
	if err == nil {
		t.Fatalf("dial without token succeeded")
	}
	if res == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want %d", responseStatus(res), http.StatusUnauthorized)
	}
}

func TestWebSocketPresenceForAcceptedContacts(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	registerUser(t, httpSrv.URL, "invite-charlie", "charlie", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")
	charlieToken := loginUser(t, httpSrv.URL, "charlie", "secret-password")
	createAcceptedContact(t, httpSrv.URL, aliceToken, bobToken, "bob")

	bobWS := dialWS(t, httpSrv.URL, bobToken)
	defer bobWS.Close()
	charlieWS := dialWS(t, httpSrv.URL, charlieToken)
	defer charlieWS.Close()

	aliceWS := dialWS(t, httpSrv.URL, aliceToken)
	event := readPresence(t, bobWS)
	if event.User.Username != "alice" || !event.Online {
		t.Fatalf("presence event = %+v, want alice online", event)
	}
	assertNoPresence(t, charlieWS)

	aliceWS.Close()
	event = readPresence(t, bobWS)
	if event.User.Username != "alice" || event.Online {
		t.Fatalf("presence event = %+v, want alice offline", event)
	}
}

func createAcceptedContact(t *testing.T, baseURL, requesterToken, recipientToken, recipientUsername string) {
	t.Helper()

	requestRes := postJSON(t, baseURL+"/api/contacts/requests", bearerHeaders(requesterToken), map[string]string{"username": recipientUsername})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)

	acceptRes := postJSON(t, baseURL+"/api/contacts/requests/"+url.PathEscape(strconv.FormatInt(request.ID, 10))+"/accept", bearerHeaders(recipientToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()
}

func dialWS(t *testing.T, baseURL, token string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(baseURL)+"/api/ws?access_token="+url.QueryEscape(token), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func readPresence(t *testing.T, conn *websocket.Conn) presenceEvent {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var event presenceEvent
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read presence: %v", err)
	}
	return event
}

func assertNoPresence(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	var event presenceEvent
	if err := conn.ReadJSON(&event); err == nil {
		t.Fatalf("unexpected presence event: %+v", event)
	}
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func responseStatus(res *http.Response) int {
	if res == nil {
		return 0
	}
	return res.StatusCode
}
