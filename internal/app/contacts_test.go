package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestContactRequestLifecycle(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-pass")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-pass")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-pass")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-pass")

	lookupRes := getJSON(t, httpSrv.URL+"/users/lookup?username="+url.QueryEscape("bob"), bearerHeaders(aliceToken))
	assertStatus(t, lookupRes, http.StatusOK)
	var lookup authUser
	decodeResponse(t, lookupRes, &lookup)
	if lookup.Username != "bob" {
		t.Fatalf("lookup username = %q, want bob", lookup.Username)
	}

	missingRes := getJSON(t, httpSrv.URL+"/users/lookup?username=missing", bearerHeaders(aliceToken))
	assertStatus(t, missingRes, http.StatusNotFound)
	missingRes.Body.Close()

	requestRes := postJSON(t, httpSrv.URL+"/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)
	if request.ID == 0 || request.Status != "pending" {
		t.Fatalf("request = %+v, want pending request with id", request)
	}

	duplicateRes := postJSON(t, httpSrv.URL+"/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, duplicateRes, http.StatusOK)
	var duplicate contactRequestResponse
	decodeResponse(t, duplicateRes, &duplicate)
	if duplicate.ID != request.ID {
		t.Fatalf("duplicate request id = %d, want %d", duplicate.ID, request.ID)
	}

	incomingRes := getJSON(t, httpSrv.URL+"/contacts/requests/incoming", bearerHeaders(bobToken))
	assertStatus(t, incomingRes, http.StatusOK)
	var incoming struct {
		Requests []contactRequestResponse `json:"requests"`
	}
	decodeResponse(t, incomingRes, &incoming)
	if len(incoming.Requests) != 1 || incoming.Requests[0].Username != "alice" {
		t.Fatalf("incoming = %+v, want one request from alice", incoming.Requests)
	}

	acceptRes := postJSON(t, httpSrv.URL+"/contacts/requests/"+url.PathEscape(strconv.FormatInt(request.ID, 10))+"/accept", bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()

	assertContactList(t, httpSrv.URL, aliceToken, "bob")
	assertContactList(t, httpSrv.URL, bobToken, "alice")
}

func TestRejectContactRequest(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-pass")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-pass")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-pass")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-pass")

	requestRes := postJSON(t, httpSrv.URL+"/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)

	rejectRes := postJSON(t, httpSrv.URL+"/contacts/requests/"+url.PathEscape(strconv.FormatInt(request.ID, 10))+"/reject", bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, rejectRes, http.StatusNoContent)
	rejectRes.Body.Close()

	contactsRes := getJSON(t, httpSrv.URL+"/contacts", bearerHeaders(aliceToken))
	assertStatus(t, contactsRes, http.StatusOK)
	var body struct {
		Contacts []authUser `json:"contacts"`
	}
	decodeResponse(t, contactsRes, &body)
	if len(body.Contacts) != 0 {
		t.Fatalf("contacts = %+v, want none after rejection", body.Contacts)
	}
}

func assertContactList(t *testing.T, baseURL, token, wantUsername string) {
	t.Helper()
	res := getJSON(t, baseURL+"/contacts", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var body struct {
		Contacts []authUser `json:"contacts"`
	}
	decodeResponse(t, res, &body)
	if len(body.Contacts) != 1 || body.Contacts[0].Username != wantUsername {
		t.Fatalf("contacts = %+v, want %q", body.Contacts, wantUsername)
	}
}

func loginUser(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	res := postJSON(t, baseURL+"/login", nil, map[string]string{
		"username": username,
		"password": password,
	})
	assertStatus(t, res, http.StatusOK)
	var body struct {
		AccessToken string `json:"access_token"`
	}
	decodeResponse(t, res, &body)
	if body.AccessToken == "" {
		t.Fatalf("access token is empty")
	}
	return body.AccessToken
}

func bearerHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func getJSON(t *testing.T, rawURL string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	return res
}
