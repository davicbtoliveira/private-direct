package app

import (
	"bytes"
	"encoding/json"
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

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")

	lookupRes := getJSON(t, httpSrv.URL+"/api/users/lookup?username="+url.QueryEscape("  BoB  "), bearerHeaders(aliceToken))
	assertStatus(t, lookupRes, http.StatusOK)
	var lookup authUser
	decodeResponse(t, lookupRes, &lookup)
	if lookup.Username != "bob" {
		t.Fatalf("lookup username = %q, want bob", lookup.Username)
	}

	missingRes := getJSON(t, httpSrv.URL+"/api/users/lookup?username=missing", bearerHeaders(aliceToken))
	assertStatus(t, missingRes, http.StatusNotFound)
	missingRes.Body.Close()

	requestRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "  BoB  "})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)
	if request.ID == 0 || request.Status != "pending" {
		t.Fatalf("request = %+v, want pending request with id", request)
	}

	duplicateRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, duplicateRes, http.StatusOK)
	var duplicate contactRequestResponse
	decodeResponse(t, duplicateRes, &duplicate)
	if duplicate.ID != request.ID {
		t.Fatalf("duplicate request id = %d, want %d", duplicate.ID, request.ID)
	}

	incomingRes := getJSON(t, httpSrv.URL+"/api/contacts/requests/incoming", bearerHeaders(bobToken))
	assertStatus(t, incomingRes, http.StatusOK)
	var incoming struct {
		Requests []contactRequestResponse `json:"requests"`
	}
	decodeResponse(t, incomingRes, &incoming)
	if len(incoming.Requests) != 1 || incoming.Requests[0].Username != "alice" {
		t.Fatalf("incoming = %+v, want one request from alice", incoming.Requests)
	}

	acceptRes := postJSON(t, httpSrv.URL+"/api/contacts/requests/"+url.PathEscape(strconv.FormatInt(request.ID, 10))+"/accept", bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()

	assertContactList(t, httpSrv.URL, aliceToken, "bob")
	assertContactList(t, httpSrv.URL, bobToken, "alice")
}

func TestReciprocalContactRequestKeepsOnePairState(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")

	requestRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)

	reciprocalRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(bobToken), map[string]string{"username": "alice"})
	assertStatus(t, reciprocalRes, http.StatusConflict)
	assertErrorCode(t, reciprocalRes, "incoming_request_exists")

	assertEmptyArrayField(t, getJSON(t, httpSrv.URL+"/api/contacts/requests/incoming", bearerHeaders(aliceToken)), "requests")
	acceptRes := postJSON(t, httpSrv.URL+"/api/contacts/requests/"+strconv.FormatInt(request.ID, 10)+"/accept", bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()

	assertContactList(t, httpSrv.URL, aliceToken, "bob")
	assertContactList(t, httpSrv.URL, bobToken, "alice")
	assertEmptyArrayField(t, getJSON(t, httpSrv.URL+"/api/contacts/requests/incoming", bearerHeaders(aliceToken)), "requests")
	assertEmptyArrayField(t, getJSON(t, httpSrv.URL+"/api/contacts/requests/incoming", bearerHeaders(bobToken)), "requests")
}

func TestConcurrentReciprocalContactRequestsCreateOnePairState(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")

	type result struct {
		status int
		code   string
		err    error
	}
	results := make(chan result, 2)
	go func() {
		status, code, err := createContactRequestResult(httpSrv.URL, aliceToken, "bob")
		results <- result{status: status, code: code, err: err}
	}()
	go func() {
		status, code, err := createContactRequestResult(httpSrv.URL, bobToken, "alice")
		results <- result{status: status, code: code, err: err}
	}()

	first, second := <-results, <-results
	if first.err != nil {
		t.Fatalf("first request: %v", first.err)
	}
	if second.err != nil {
		t.Fatalf("second request: %v", second.err)
	}
	created := 0
	conflicts := 0
	for _, got := range []result{first, second} {
		switch {
		case got.status == http.StatusCreated:
			created++
		case got.status == http.StatusConflict && got.code == "incoming_request_exists":
			conflicts++
		default:
			t.Fatalf("request result = %+v, want one created and one incoming conflict", got)
		}
	}
	if created != 1 || conflicts != 1 {
		t.Fatalf("created = %d, conflicts = %d, want 1 each", created, conflicts)
	}

	if total := incomingRequestCount(t, httpSrv.URL, aliceToken) + incomingRequestCount(t, httpSrv.URL, bobToken); total != 1 {
		t.Fatalf("total incoming requests = %d, want 1", total)
	}
}

func TestContactLookupExcludesSelf(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")

	lookupRes := getJSON(t, httpSrv.URL+"/api/users/lookup?username="+url.QueryEscape(" Alice "), bearerHeaders(aliceToken))
	assertStatus(t, lookupRes, http.StatusNotFound)
	assertErrorCode(t, lookupRes, "user_not_found")

	requestRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": " ALICE "})
	assertStatus(t, requestRes, http.StatusBadRequest)
	assertErrorCode(t, requestRes, "cannot_contact_self")
}

func TestContactCollectionsAreArraysAndContactsAreSorted(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-zoe", "zoe", "secret-password")
	registerUser(t, httpSrv.URL, "invite-amy", "amy", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	zoeToken := loginUser(t, httpSrv.URL, "zoe", "secret-password")
	amyToken := loginUser(t, httpSrv.URL, "amy", "secret-password")

	assertEmptyArrayField(t, getJSON(t, httpSrv.URL+"/api/contacts", bearerHeaders(aliceToken)), "contacts")
	assertEmptyArrayField(t, getJSON(t, httpSrv.URL+"/api/contacts/requests/incoming", bearerHeaders(aliceToken)), "requests")

	requestAndAccept(t, httpSrv.URL, zoeToken, aliceToken, "alice")
	requestAndAccept(t, httpSrv.URL, amyToken, aliceToken, "alice")

	contactsRes := getJSON(t, httpSrv.URL+"/api/contacts", bearerHeaders(aliceToken))
	assertStatus(t, contactsRes, http.StatusOK)
	var contacts struct {
		Contacts []authUser `json:"contacts"`
	}
	decodeResponse(t, contactsRes, &contacts)
	if len(contacts.Contacts) != 2 || contacts.Contacts[0].Username != "amy" || contacts.Contacts[1].Username != "zoe" {
		t.Fatalf("contacts = %+v, want amy then zoe", contacts.Contacts)
	}

	assertEmptyArrayField(t, getJSON(t, httpSrv.URL+"/api/contacts/requests/incoming", bearerHeaders(aliceToken)), "requests")
}

func TestOnlyRecipientCanResolveContactRequest(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	registerUser(t, httpSrv.URL, "invite-carol", "carol", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")
	carolToken := loginUser(t, httpSrv.URL, "carol", "secret-password")

	requestRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)

	acceptURL := httpSrv.URL + "/api/contacts/requests/" + strconv.FormatInt(request.ID, 10) + "/accept"
	wrongRecipientRes := postJSON(t, acceptURL, bearerHeaders(carolToken), map[string]string{})
	assertStatus(t, wrongRecipientRes, http.StatusNotFound)
	assertErrorCode(t, wrongRecipientRes, "request_not_found")

	acceptRes := postJSON(t, acceptURL, bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()
	assertContactList(t, httpSrv.URL, aliceToken, "bob")
}

func TestRejectContactRequest(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	registerUser(t, httpSrv.URL, "invite-bob", "bob", "secret-password")
	aliceToken := loginUser(t, httpSrv.URL, "alice", "secret-password")
	bobToken := loginUser(t, httpSrv.URL, "bob", "secret-password")

	requestRes := postJSON(t, httpSrv.URL+"/api/contacts/requests", bearerHeaders(aliceToken), map[string]string{"username": "bob"})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)

	rejectRes := postJSON(t, httpSrv.URL+"/api/contacts/requests/"+url.PathEscape(strconv.FormatInt(request.ID, 10))+"/reject", bearerHeaders(bobToken), map[string]string{})
	assertStatus(t, rejectRes, http.StatusNoContent)
	rejectRes.Body.Close()

	contactsRes := getJSON(t, httpSrv.URL+"/api/contacts", bearerHeaders(aliceToken))
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
	res := getJSON(t, baseURL+"/api/contacts", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var body struct {
		Contacts []authUser `json:"contacts"`
	}
	decodeResponse(t, res, &body)
	if len(body.Contacts) != 1 || body.Contacts[0].Username != wantUsername {
		t.Fatalf("contacts = %+v, want %q", body.Contacts, wantUsername)
	}
}

func requestAndAccept(t *testing.T, baseURL, requesterToken, recipientToken, recipientUsername string) {
	t.Helper()
	requestRes := postJSON(t, baseURL+"/api/contacts/requests", bearerHeaders(requesterToken), map[string]string{"username": recipientUsername})
	assertStatus(t, requestRes, http.StatusCreated)
	var request contactRequestResponse
	decodeResponse(t, requestRes, &request)

	acceptRes := postJSON(t, baseURL+"/api/contacts/requests/"+strconv.FormatInt(request.ID, 10)+"/accept", bearerHeaders(recipientToken), map[string]string{})
	assertStatus(t, acceptRes, http.StatusNoContent)
	acceptRes.Body.Close()
}

func assertEmptyArrayField(t *testing.T, res *http.Response, field string) {
	t.Helper()
	assertStatus(t, res, http.StatusOK)
	var body map[string]json.RawMessage
	decodeResponse(t, res, &body)
	if string(body[field]) != "[]" {
		t.Fatalf("%s = %s, want []", field, body[field])
	}
}

func incomingRequestCount(t *testing.T, baseURL, token string) int {
	t.Helper()
	res := getJSON(t, baseURL+"/api/contacts/requests/incoming", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var body struct {
		Requests []contactRequestResponse `json:"requests"`
	}
	decodeResponse(t, res, &body)
	return len(body.Requests)
}

func createContactRequestResult(baseURL, token, username string) (int, string, error) {
	data, err := json.Marshal(map[string]string{"username": username})
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/contacts/requests", bytes.NewReader(data))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return 0, "", err
	}
	return res.StatusCode, body.Error, nil
}

func loginUser(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	res := postJSON(t, baseURL+"/api/login", nil, map[string]string{
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
