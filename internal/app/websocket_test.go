package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketRequiresExactSubprotocolCredentials(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")

	tests := []struct {
		name      string
		protocols []string
		path      string
		header    http.Header
	}{
		{name: "missing protocols", path: "/api/ws"},
		{name: "application protocol only", protocols: []string{webSocketProtocol}, path: "/api/ws"},
		{name: "token only", protocols: []string{token}, path: "/api/ws"},
		{name: "reversed protocols", protocols: []string{token, webSocketProtocol}, path: "/api/ws"},
		{name: "extra protocol", protocols: []string{webSocketProtocol, token, "extra"}, path: "/api/ws"},
		{name: "invalid token", protocols: []string{webSocketProtocol, "not-a-token"}, path: "/api/ws"},
		{name: "query token", path: "/api/ws?access_token=" + url.QueryEscape(token)},
		{name: "query token with protocols", protocols: []string{webSocketProtocol, token}, path: "/api/ws?access_token=" + url.QueryEscape(token)},
		{name: "authorization header", path: "/api/ws", header: http.Header{"Authorization": []string{"Bearer " + token}}},
		{name: "authorization header with protocols", protocols: []string{webSocketProtocol, token}, path: "/api/ws", header: http.Header{"Authorization": []string{"Bearer " + token}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer := websocket.Dialer{Subprotocols: tt.protocols}
			conn, res, err := dialer.Dial(wsURL(httpSrv.URL)+tt.path, tt.header)
			if conn != nil {
				conn.Close()
			}
			if err == nil {
				t.Fatalf("dial succeeded")
			}
			if res == nil || res.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %v, want %d", responseStatus(res), http.StatusUnauthorized)
			}
			res.Body.Close()
		})
	}
}

func TestWebSocketPresenceSnapshotAndDeltas(t *testing.T) {
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

	bobWS, bobSnapshot := dialWSWithSnapshot(t, httpSrv.URL, bobToken)
	defer bobWS.Close()
	if len(bobSnapshot.OnlineUsers) != 0 {
		t.Fatalf("bob snapshot = %+v, want empty", bobSnapshot.OnlineUsers)
	}
	charlieWS, charlieSnapshot := dialWSWithSnapshot(t, httpSrv.URL, charlieToken)
	defer charlieWS.Close()
	if len(charlieSnapshot.OnlineUsers) != 0 {
		t.Fatalf("charlie snapshot = %+v, want empty", charlieSnapshot.OnlineUsers)
	}

	aliceWS, aliceSnapshot := dialWSWithSnapshot(t, httpSrv.URL, aliceToken)
	if len(aliceSnapshot.OnlineUsers) != 1 || aliceSnapshot.OnlineUsers[0].Username != "bob" {
		t.Fatalf("alice snapshot = %+v, want online bob only", aliceSnapshot.OnlineUsers)
	}
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

func TestWebSocketReplacementKeepsPresenceAndDisablesOldClient(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")
	bobID := lookupUserID(t, httpSrv.URL, aliceToken, "bob")
	createAcceptedContact(t, httpSrv.URL, aliceToken, bobToken, "bob")

	bobWS := dialWS(t, httpSrv.URL, bobToken)
	defer bobWS.Close()
	oldAliceWS := dialWS(t, httpSrv.URL, aliceToken)
	defer oldAliceWS.Close()
	online := readPresence(t, bobWS)
	if online.User.Username != "alice" || !online.Online {
		t.Fatalf("presence event = %+v, want alice online", online)
	}

	newAliceWS, snapshot := dialWSWithSnapshot(t, httpSrv.URL, aliceToken)
	defer newAliceWS.Close()
	if len(snapshot.OnlineUsers) != 1 || snapshot.OnlineUsers[0].Username != "bob" {
		t.Fatalf("replacement snapshot = %+v, want online bob", snapshot.OnlineUsers)
	}
	eventType, _ := readRawWSEvent(t, oldAliceWS)
	if eventType != "session_replaced" {
		t.Fatalf("old client event = %q, want session_replaced", eventType)
	}

	assertContactList(t, httpSrv.URL, aliceToken, "bob")
	oldAliceWS.SetWriteDeadline(time.Now().Add(time.Second))
	_ = oldAliceWS.WriteJSON(map[string]any{
		"type":        "signal",
		"to_user_id":  bobID,
		"signal_type": "offer",
		"payload":     map[string]string{"sdp": "old-client"},
	})
	writeWS(t, newAliceWS, map[string]any{
		"type":        "signal",
		"to_user_id":  bobID,
		"signal_type": "offer",
		"payload":     map[string]string{"sdp": "new-client"},
	})

	eventType, data := readRawWSEvent(t, bobWS)
	if eventType != "signal" {
		t.Fatalf("bob event after replacement = %q, want signal without presence flicker", eventType)
	}
	var signal signalEvent
	if err := json.Unmarshal(data, &signal); err != nil {
		t.Fatalf("decode signal: %v", err)
	}
	if string(signal.Payload) != `{"sdp":"new-client"}` {
		t.Fatalf("signal payload = %s, want new client only", signal.Payload)
	}

	newAliceWS.Close()
	offline := readPresence(t, bobWS)
	if offline.User.Username != "alice" || offline.Online {
		t.Fatalf("presence event = %+v, want alice offline", offline)
	}
}

func TestWebSocketContactInvalidationsFollowCommittedState(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	registerUser(t, httpSrv.URL, "invite-charlie", "charlie", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")
	charlieToken := loginUser(t, httpSrv.URL, "charlie", "secret-password")

	aliceWS := dialWS(t, httpSrv.URL, aliceToken)
	defer aliceWS.Close()
	bobWS := dialWS(t, httpSrv.URL, bobToken)
	defer bobWS.Close()
	charlieWS := dialWS(t, httpSrv.URL, charlieToken)
	defer charlieWS.Close()

	bobRequest := createContactRequest(t, httpSrv.URL, aliceToken, "bob")
	assertNextWSEventType(t, bobWS, "contacts_changed")
	if got := incomingRequestCount(t, httpSrv.URL, bobToken); got != 1 {
		t.Fatalf("bob incoming requests = %d, want 1 after invalidation", got)
	}
	rejectRes := postJSON(t, httpSrv.URL+"/api/contacts/requests/"+strconv.FormatInt(bobRequest.ID, 10)+"/reject", bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, rejectRes, http.StatusNoContent)
	rejectRes.Body.Close()
	assertNextWSEventType(t, aliceWS, "contacts_changed")
	assertNextWSEventType(t, bobWS, "contacts_changed")
	if got := incomingRequestCount(t, httpSrv.URL, bobToken); got != 0 {
		t.Fatalf("bob incoming requests = %d, want 0 after rejection", got)
	}

	charlieRequest := createContactRequest(t, httpSrv.URL, aliceToken, "charlie")
	assertNextWSEventType(t, charlieWS, "contacts_changed")
	if got := incomingRequestCount(t, httpSrv.URL, charlieToken); got != 1 {
		t.Fatalf("charlie incoming requests = %d, want 1 after invalidation", got)
	}
	acceptRes := postJSON(t, httpSrv.URL+"/api/contacts/requests/"+strconv.FormatInt(charlieRequest.ID, 10)+"/accept", bearerHeaders(charlieToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()

	assertNextWSEventType(t, aliceWS, "contacts_changed")
	alicePresence := readPresence(t, aliceWS)
	if alicePresence.User.Username != "charlie" || !alicePresence.Online {
		t.Fatalf("alice presence = %+v, want charlie online", alicePresence)
	}
	assertNextWSEventType(t, charlieWS, "contacts_changed")
	charliePresence := readPresence(t, charlieWS)
	if charliePresence.User.Username != "alice" || !charliePresence.Online {
		t.Fatalf("charlie presence = %+v, want alice online", charliePresence)
	}
	assertContactList(t, httpSrv.URL, aliceToken, "charlie")
	assertContactList(t, httpSrv.URL, charlieToken, "alice")
}

func TestPresenceHubWriteFailureRemovesClient(t *testing.T) {
	hub := newPresenceHub()
	conn := &failSecondWriteConn{}
	removed := make(chan struct{}, 1)
	client := &wsClient{
		user: authUser{ID: 1, Username: "alice"},
		conn: conn,
	}
	client.onWriteFailure = func(failed *wsClient) {
		if hub.remove(failed) {
			removed <- struct{}{}
		}
	}
	becameOnline, err := hub.activate(client, func() ([]authUser, error) {
		return []authUser{}, nil
	})
	if err != nil || !becameOnline {
		t.Fatalf("activate = %t, %v, want newly online", becameOnline, err)
	}
	if hub.sendToUser(client.user.ID, contactsChangedEvent{Type: "contacts_changed"}) {
		t.Fatalf("failed write reported success")
	}
	processed := false
	if client.processMessage(func() { processed = true }) || processed {
		t.Fatalf("write-failed client still processed inbound message")
	}
	select {
	case <-removed:
	case <-time.After(time.Second):
		t.Fatalf("failed client was not removed")
	}
	if hub.isOnline(client.user.ID) {
		t.Fatalf("failed client remains online")
	}
}

func TestReplacementWaitsForOldMessageBeforeSwitchingClient(t *testing.T) {
	hub := newPresenceHub()
	oldClient := &wsClient{
		user: authUser{ID: 1, Username: "alice"},
		conn: &successfulWSConn{},
	}
	if _, err := hub.activate(oldClient, func() ([]authUser, error) { return []authUser{}, nil }); err != nil {
		t.Fatalf("activate old client: %v", err)
	}
	messageStarted := make(chan struct{})
	releaseMessage := make(chan struct{})
	messageDone := make(chan struct{})
	go func() {
		oldClient.processMessage(func() {
			close(messageStarted)
			<-releaseMessage
		})
		close(messageDone)
	}()
	<-messageStarted

	newConn := newRecordingWSConn()
	newClient := &wsClient{
		user: oldClient.user,
		conn: newConn,
	}
	replacementDone := make(chan struct{})
	go func() {
		if _, err := hub.activate(newClient, func() ([]authUser, error) { return []authUser{}, nil }); err != nil {
			t.Errorf("activate new client: %v", err)
		}
		close(replacementDone)
	}()
	deadline := time.Now().Add(time.Second)
	for oldClient.active.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if oldClient.active.Load() {
		t.Fatalf("replacement did not revoke old client")
	}
	select {
	case <-replacementDone:
		t.Fatalf("replacement completed while old message was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	hub.mu.RLock()
	current := hub.clients[oldClient.user.ID]
	hub.mu.RUnlock()
	if current != oldClient {
		t.Fatalf("active client switched before old message completed")
	}
	routed := make(chan bool, 1)
	go func() {
		routed <- hub.sendToUser(oldClient.user.ID, signalEvent{
			Type:       "signal",
			From:       authUser{ID: 2, Username: "bob"},
			SignalType: "offer",
			Payload:    json.RawMessage(`{"sdp":"new-client"}`),
		})
	}()
	close(releaseMessage)
	<-messageDone
	<-replacementDone
	if !<-routed {
		t.Fatalf("signal was not routed to snapshot-ready replacement")
	}
	hub.mu.RLock()
	current = hub.clients[oldClient.user.ID]
	hub.mu.RUnlock()
	if current != newClient {
		t.Fatalf("new client was not activated")
	}
	if _, ok := nextRecordedEvent(t, newConn).(presenceSnapshotEvent); !ok {
		t.Fatalf("replacement first event was not presence snapshot")
	}
	if _, ok := nextRecordedEvent(t, newConn).(signalEvent); !ok {
		t.Fatalf("replacement did not receive queued signal")
	}

	relayed := false
	if oldClient.processMessage(func() { relayed = true }) || relayed {
		t.Fatalf("replaced client processed another signal")
	}
}

func TestPresenceHubKeepsSnapshotFirstWhenInvalidatedDuringLoad(t *testing.T) {
	hub := newPresenceHub()
	conn := newRecordingWSConn()
	client := &wsClient{
		user: authUser{ID: 1, Username: "alice"},
		conn: conn,
	}
	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	activationDone := make(chan error, 1)
	go func() {
		_, err := hub.activate(client, func() ([]authUser, error) {
			close(loadStarted)
			<-releaseLoad
			return []authUser{}, nil
		})
		activationDone <- err
	}()
	<-loadStarted
	hub.notifyUser(client.user.ID, contactsChangedEvent{Type: "contacts_changed"})
	close(releaseLoad)
	if err := <-activationDone; err != nil {
		t.Fatalf("activate client: %v", err)
	}
	defer client.close()

	if _, ok := nextRecordedEvent(t, conn).(presenceSnapshotEvent); !ok {
		t.Fatalf("first event was not presence snapshot")
	}
	if _, ok := nextRecordedEvent(t, conn).(contactsChangedEvent); !ok {
		t.Fatalf("second event was not contacts_changed")
	}
}

func TestConcurrentTakeoversDoNotDeadlockCrossSignals(t *testing.T) {
	hub := newPresenceHub()
	aliceOld := &wsClient{user: authUser{ID: 1, Username: "alice"}, conn: &successfulWSConn{}}
	bobOld := &wsClient{user: authUser{ID: 2, Username: "bob"}, conn: &successfulWSConn{}}
	if _, err := hub.activate(aliceOld, func() ([]authUser, error) { return []authUser{bobOld.user}, nil }); err != nil {
		t.Fatalf("activate old alice: %v", err)
	}
	if _, err := hub.activate(bobOld, func() ([]authUser, error) { return []authUser{aliceOld.user}, nil }); err != nil {
		t.Fatalf("activate old bob: %v", err)
	}
	defer aliceOld.close()
	defer bobOld.close()

	messageReady := make(chan struct{}, 2)
	releaseMessages := make(chan struct{})
	messagesDone := make(chan struct{}, 2)
	startOldMessage := func(source *wsClient, targetID int64) {
		go func() {
			source.processMessage(func() {
				messageReady <- struct{}{}
				<-releaseMessages
				if !hub.sendToUser(targetID, signalEvent{
					Type:       "signal",
					From:       source.user,
					SignalType: "offer",
					Payload:    json.RawMessage(`{"sdp":"cross"}`),
				}) {
					t.Errorf("cross signal from %s was not queued", source.user.Username)
				}
			})
			messagesDone <- struct{}{}
		}()
	}
	startOldMessage(aliceOld, bobOld.user.ID)
	startOldMessage(bobOld, aliceOld.user.ID)
	<-messageReady
	<-messageReady

	aliceNewConn := newRecordingWSConn()
	bobNewConn := newRecordingWSConn()
	aliceNew := &wsClient{user: aliceOld.user, conn: aliceNewConn}
	bobNew := &wsClient{user: bobOld.user, conn: bobNewConn}
	takeoversDone := make(chan error, 2)
	go func() {
		_, err := hub.activate(aliceNew, func() ([]authUser, error) { return []authUser{bobOld.user}, nil })
		takeoversDone <- err
	}()
	go func() {
		_, err := hub.activate(bobNew, func() ([]authUser, error) { return []authUser{aliceOld.user}, nil })
		takeoversDone <- err
	}()
	waitForClientRevoked(t, aliceOld)
	waitForClientRevoked(t, bobOld)
	close(releaseMessages)

	for range 2 {
		select {
		case <-messagesDone:
		case <-time.After(time.Second):
			t.Fatalf("cross signaling deadlocked old message handlers")
		}
	}
	for range 2 {
		if err := <-takeoversDone; err != nil {
			t.Fatalf("takeover failed: %v", err)
		}
	}
	defer aliceNew.close()
	defer bobNew.close()
	if _, ok := nextRecordedEvent(t, aliceNewConn).(presenceSnapshotEvent); !ok {
		t.Fatalf("alice replacement first event was not snapshot")
	}
	if _, ok := nextRecordedEvent(t, aliceNewConn).(signalEvent); !ok {
		t.Fatalf("alice replacement did not receive bob signal")
	}
	if _, ok := nextRecordedEvent(t, bobNewConn).(presenceSnapshotEvent); !ok {
		t.Fatalf("bob replacement first event was not snapshot")
	}
	if _, ok := nextRecordedEvent(t, bobNewConn).(signalEvent); !ok {
		t.Fatalf("bob replacement did not receive alice signal")
	}
}

func TestPresenceHubQueuesConnectingPeerDeltaAfterSnapshot(t *testing.T) {
	hub := newPresenceHub()
	aliceConn := newRecordingWSConn()
	aliceConn.blockFirst = true
	alice := &wsClient{
		user: authUser{ID: 1, Username: "alice"},
		conn: aliceConn,
	}
	bob := &wsClient{
		user: authUser{ID: 2, Username: "bob"},
		conn: newRecordingWSConn(),
	}
	activationDone := make(chan error, 1)
	go func() {
		_, err := hub.activate(alice, func() ([]authUser, error) {
			return []authUser{bob.user}, nil
		})
		activationDone <- err
	}()
	<-aliceConn.firstWriteStarted

	if _, err := hub.activate(bob, func() ([]authUser, error) {
		return []authUser{alice.user}, nil
	}); err != nil {
		t.Fatalf("activate bob: %v", err)
	}
	close(aliceConn.releaseFirstWrite)
	if err := <-activationDone; err != nil {
		t.Fatalf("activate alice: %v", err)
	}
	defer alice.close()
	defer bob.close()

	snapshot, ok := nextRecordedEvent(t, aliceConn).(presenceSnapshotEvent)
	if !ok {
		t.Fatalf("first event was not presence snapshot")
	}
	if len(snapshot.OnlineUsers) != 0 {
		t.Fatalf("alice snapshot = %+v, want bob delta after snapshot", snapshot.OnlineUsers)
	}
	delta, ok := nextRecordedEvent(t, aliceConn).(presenceEvent)
	if !ok || delta.User != bob.user || !delta.Online {
		t.Fatalf("alice delta = %+v, want bob online", delta)
	}
}

func TestPresenceHubAcceptDuringInitializationUsesCommittedPeers(t *testing.T) {
	hub := newPresenceHub()
	bobConn := newRecordingWSConn()
	bob := &wsClient{
		user: authUser{ID: 2, Username: "bob"},
		conn: bobConn,
	}
	if _, err := hub.activate(bob, func() ([]authUser, error) { return []authUser{}, nil }); err != nil {
		t.Fatalf("activate bob: %v", err)
	}
	defer bob.close()
	_ = nextRecordedEvent(t, bobConn)

	aliceConn := newRecordingWSConn()
	alice := &wsClient{
		user: authUser{ID: 1, Username: "alice"},
		conn: aliceConn,
	}
	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	activationDone := make(chan error, 1)
	go func() {
		_, err := hub.activate(alice, func() ([]authUser, error) {
			close(loadStarted)
			<-releaseLoad
			return []authUser{}, nil
		})
		activationDone <- err
	}()
	<-loadStarted

	hub.sendCurrentPresence(alice.user.ID, bob.user)
	hub.sendCurrentPresence(bob.user.ID, alice.user)
	close(releaseLoad)
	if err := <-activationDone; err != nil {
		t.Fatalf("activate alice: %v", err)
	}
	defer alice.close()

	snapshot, ok := nextRecordedEvent(t, aliceConn).(presenceSnapshotEvent)
	if !ok || len(snapshot.OnlineUsers) != 1 || snapshot.OnlineUsers[0] != bob.user {
		t.Fatalf("alice snapshot = %+v, want accepted online bob", snapshot)
	}
	assertRecordedPresence(t, bobConn, alice.user, false)
	assertRecordedPresence(t, bobConn, alice.user, true)
}

func TestPresenceHubSnapshotFailureDoesNotPublishOnline(t *testing.T) {
	hub := newPresenceHub()
	bobConn := newRecordingWSConn()
	bob := &wsClient{
		user: authUser{ID: 2, Username: "bob"},
		conn: bobConn,
	}
	if _, err := hub.activate(bob, func() ([]authUser, error) { return []authUser{}, nil }); err != nil {
		t.Fatalf("activate bob: %v", err)
	}
	defer bob.close()
	_ = nextRecordedEvent(t, bobConn)

	alice := &wsClient{
		user: authUser{ID: 1, Username: "alice"},
		conn: &failFirstWriteConn{},
	}
	if _, err := hub.activate(alice, func() ([]authUser, error) {
		return []authUser{bob.user}, nil
	}); !errors.Is(err, errPresenceSnapshotWrite) {
		t.Fatalf("activate alice error = %v, want snapshot write failure", err)
	}
	assertNoRecordedEvent(t, bobConn)
	if hub.isOnline(alice.user.ID) {
		t.Fatalf("failed snapshot published alice online")
	}
}

func TestCurrentPresenceCannotOverwriteNewerTransition(t *testing.T) {
	t.Run("online then offline", func(t *testing.T) {
		hub := newPresenceHub()
		alice := &wsClient{user: authUser{ID: 1, Username: "alice"}, conn: newRecordingWSConn()}
		bobConn := newRecordingWSConn()
		bob := &wsClient{user: authUser{ID: 2, Username: "bob"}, conn: bobConn}
		if _, err := hub.activate(alice, func() ([]authUser, error) { return []authUser{}, nil }); err != nil {
			t.Fatalf("activate alice: %v", err)
		}
		if _, err := hub.activate(bob, func() ([]authUser, error) { return []authUser{alice.user}, nil }); err != nil {
			t.Fatalf("activate bob: %v", err)
		}
		defer alice.close()
		defer bob.close()
		_ = nextRecordedEvent(t, bobConn)

		hub.sendCurrentPresence(bob.user.ID, alice.user)
		if !hub.remove(alice) {
			t.Fatalf("alice removal did not transition offline")
		}
		hub.sendPresenceToContacts(alice.user, false, []int64{bob.user.ID})
		assertRecordedPresence(t, bobConn, alice.user, true)
		assertRecordedPresence(t, bobConn, alice.user, false)
	})

	t.Run("offline then online", func(t *testing.T) {
		hub := newPresenceHub()
		alice := &wsClient{user: authUser{ID: 1, Username: "alice"}, conn: newRecordingWSConn()}
		bobConn := newRecordingWSConn()
		bob := &wsClient{user: authUser{ID: 2, Username: "bob"}, conn: bobConn}
		if _, err := hub.activate(bob, func() ([]authUser, error) { return []authUser{alice.user}, nil }); err != nil {
			t.Fatalf("activate bob: %v", err)
		}
		defer alice.close()
		defer bob.close()
		_ = nextRecordedEvent(t, bobConn)

		hub.sendCurrentPresence(bob.user.ID, alice.user)
		if _, err := hub.activate(alice, func() ([]authUser, error) { return []authUser{bob.user}, nil }); err != nil {
			t.Fatalf("activate alice: %v", err)
		}
		assertRecordedPresence(t, bobConn, alice.user, false)
		assertRecordedPresence(t, bobConn, alice.user, true)
	})
}

func createAcceptedContact(t *testing.T, baseURL, requesterToken, recipientToken, recipientUsername string) {
	t.Helper()
	request := createContactRequest(t, baseURL, requesterToken, recipientUsername)
	acceptRes := postJSON(t, baseURL+"/api/contacts/requests/"+url.PathEscape(strconv.FormatInt(request.ID, 10))+"/accept", bearerHeaders(recipientToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()
}

func createContactRequest(t *testing.T, baseURL, requesterToken, recipientUsername string) contactRequestResponse {
	t.Helper()
	requestRes := postJSON(t, baseURL+"/api/contacts/requests", bearerHeaders(requesterToken), map[string]string{"username": recipientUsername})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)
	return request
}

func dialWS(t *testing.T, baseURL, token string) *websocket.Conn {
	t.Helper()
	conn, _ := dialWSWithSnapshot(t, baseURL, token)
	return conn
}

func dialWSWithSnapshot(t *testing.T, baseURL, token string) (*websocket.Conn, presenceSnapshotEvent) {
	t.Helper()
	dialer := websocket.Dialer{Subprotocols: []string{webSocketProtocol, token}}
	conn, res, err := dialer.Dial(wsURL(baseURL)+"/api/ws", nil)
	if err != nil {
		t.Fatalf("dial websocket: %v (status %d)", err, responseStatus(res))
	}
	if conn.Subprotocol() != webSocketProtocol {
		conn.Close()
		t.Fatalf("negotiated protocol = %q, want %q", conn.Subprotocol(), webSocketProtocol)
	}
	return conn, readPresenceSnapshot(t, conn)
}

func readPresenceSnapshot(t *testing.T, conn *websocket.Conn) presenceSnapshotEvent {
	t.Helper()
	eventType, data := readRawWSEvent(t, conn)
	if eventType != "presence_snapshot" {
		t.Fatalf("first websocket event = %q, want presence_snapshot", eventType)
	}
	var snapshot presenceSnapshotEvent
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("decode presence snapshot: %v", err)
	}
	if snapshot.OnlineUsers == nil {
		t.Fatalf("presence snapshot online_users = null, want []")
	}
	return snapshot
}

func readPresence(t *testing.T, conn *websocket.Conn) presenceEvent {
	t.Helper()
	eventType, data := readRawWSEvent(t, conn)
	if eventType != "presence" {
		t.Fatalf("websocket event = %q, want presence", eventType)
	}
	var event presenceEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("decode presence: %v", err)
	}
	return event
}

func assertNextWSEventType(t *testing.T, conn *websocket.Conn, want string) {
	t.Helper()
	eventType, _ := readRawWSEvent(t, conn)
	if eventType != want {
		t.Fatalf("websocket event = %q, want %q", eventType, want)
	}
}

func assertNoPresence(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var event map[string]any
	if err := conn.ReadJSON(&event); err == nil {
		t.Fatalf("unexpected websocket event: %+v", event)
	}
}

func waitForClientRevoked(t *testing.T, client *wsClient) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for client.active.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if client.active.Load() {
		t.Fatalf("client %d was not revoked", client.user.ID)
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

type failSecondWriteConn struct {
	mu     sync.Mutex
	writes int
}

func (c *failSecondWriteConn) WriteJSON(any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	if c.writes == 2 {
		return errors.New("write failed")
	}
	return nil
}

func (c *failSecondWriteConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *failSecondWriteConn) Close() error {
	return nil
}

type successfulWSConn struct{}

func (c *successfulWSConn) WriteJSON(any) error {
	return nil
}

func (c *successfulWSConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *successfulWSConn) Close() error {
	return nil
}

type recordingWSConn struct {
	mu                sync.Mutex
	events            chan any
	blockFirst        bool
	firstWriteStarted chan struct{}
	releaseFirstWrite chan struct{}
	writes            int
}

func newRecordingWSConn() *recordingWSConn {
	return &recordingWSConn{
		events:            make(chan any, 16),
		firstWriteStarted: make(chan struct{}),
		releaseFirstWrite: make(chan struct{}),
	}
}

func (c *recordingWSConn) WriteJSON(message any) error {
	c.mu.Lock()
	c.writes++
	isFirst := c.writes == 1
	block := isFirst && c.blockFirst
	c.mu.Unlock()
	if block {
		close(c.firstWriteStarted)
		<-c.releaseFirstWrite
	}
	c.events <- message
	return nil
}

func (c *recordingWSConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *recordingWSConn) Close() error {
	return nil
}

type failFirstWriteConn struct{}

func (c *failFirstWriteConn) WriteJSON(any) error {
	return errors.New("write failed")
}

func (c *failFirstWriteConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *failFirstWriteConn) Close() error {
	return nil
}

func nextRecordedEvent(t *testing.T, conn *recordingWSConn) any {
	t.Helper()
	select {
	case event := <-conn.events:
		return event
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for recorded websocket event")
		return nil
	}
}

func assertNoRecordedEvent(t *testing.T, conn *recordingWSConn) {
	t.Helper()
	select {
	case event := <-conn.events:
		t.Fatalf("unexpected websocket event: %+v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func assertRecordedPresence(t *testing.T, conn *recordingWSConn, user authUser, online bool) {
	t.Helper()
	event, ok := nextRecordedEvent(t, conn).(presenceEvent)
	if !ok || event.User != user || event.Online != online {
		t.Fatalf("presence event = %+v, want user %+v online=%t", event, user, online)
	}
}
