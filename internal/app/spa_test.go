package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertSPAResponse(t *testing.T, res *http.Response) {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want SPA 200", res.StatusCode)
	}
	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(body), `id="root"`) {
		t.Fatalf("body does not contain SPA root marker")
	}
}

func TestSPAFallbackForClientRoutes(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	for _, route := range []string{"/login", "/register", "/chat/alice", "/chat/alice.b"} {
		res, err := http.Get(httpSrv.URL + route)
		if err != nil {
			t.Fatalf("GET %s: %v", route, err)
		}
		assertSPAResponse(t, res)
	}
}

func TestUnknownAPIRouteReturnsReal404(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res, err := http.Get(httpSrv.URL + "/api/nope")
	if err != nil {
		t.Fatalf("GET /api/nope: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), `id="root"`) {
		t.Fatalf("unknown /api route returned the SPA fallback")
	}
}

func TestMissingAssetReturnsReal404(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res, err := http.Get(httpSrv.URL + "/assets/missing.js")
	if err != nil {
		t.Fatalf("GET /assets/missing.js: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}

func TestNonGetFallbackReturnsReal404(t *testing.T) {
	srv := newTestServer(t)
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	res, err := http.Post(httpSrv.URL+"/login", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for non-GET client route", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if strings.Contains(string(body), `id="root"`) {
		t.Fatalf("non-GET fallback returned the SPA")
	}
}
