package app

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestICEServersSTUNOnly(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")

	res := getJSON(t, httpSrv.URL+"/api/ice-servers", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var body struct {
		ICEServers []ICEServer `json:"ice_servers"`
	}
	decodeResponse(t, res, &body)
	if len(body.ICEServers) != 1 {
		t.Fatalf("ice servers = %+v, want STUN only", body.ICEServers)
	}
	if len(body.ICEServers[0].URLs) != 1 || body.ICEServers[0].URLs[0] != "stun:test.example" {
		t.Fatalf("STUN urls = %+v, want stun:test.example", body.ICEServers[0].URLs)
	}
}

func TestICEServersWithTURN(t *testing.T) {
	srv := newTestServerWithConfig(t, Config{
		Addr:          "127.0.0.1:0",
		DatabasePath:  filepath.Join(t.TempDir(), "private-direct.db"),
		OperatorToken: "operator-secret",
		JWTSecret:     "test-jwt-secret",
		STUNServers:   []string{"stun:test.example"},
		TURNServers: []ICEServer{
			{
				URLs:       []string{"turn:test.example"},
				Username:   "turn-user",
				Credential: "turn-pass",
			},
		},
	})
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()
	registerUser(t, httpSrv.URL, "invite-alice", "alice", "secret-password")
	token := loginUser(t, httpSrv.URL, "alice", "secret-password")

	res := getJSON(t, httpSrv.URL+"/api/ice-servers", bearerHeaders(token))
	assertStatus(t, res, http.StatusOK)
	var body struct {
		ICEServers []ICEServer `json:"ice_servers"`
	}
	decodeResponse(t, res, &body)
	if len(body.ICEServers) != 2 {
		t.Fatalf("ice servers = %+v, want STUN + TURN", body.ICEServers)
	}
	if body.ICEServers[1].Username != "turn-user" || body.ICEServers[1].Credential != "turn-pass" {
		t.Fatalf("TURN server = %+v, want configured credentials", body.ICEServers[1])
	}
}

func TestICEServersRequireAuth(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res := getJSON(t, httpSrv.URL+"/api/ice-servers", nil)
	assertStatus(t, res, http.StatusUnauthorized)
	res.Body.Close()
}

func TestConfigRequiresSTUN(t *testing.T) {
	_, err := NewServer(Config{
		Addr:          "127.0.0.1:0",
		DatabasePath:  filepath.Join(t.TempDir(), "private-direct.db"),
		OperatorToken: "operator-secret",
		JWTSecret:     "test-jwt-secret",
	})
	if err == nil {
		t.Fatalf("NewServer without STUN succeeded")
	}
}
