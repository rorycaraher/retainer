package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"retainer/server/internal/authsvc"
	"retainer/server/internal/db"
	"retainer/server/internal/wsserver"
)

// newTestServer uses TLS because the session cookie is marked Secure; a plain
// httptest.Server + cookiejar would silently drop it and break every
// authenticated request after login.
func newTestServer(t *testing.T, password string) (*httptest.Server, *sql.DB) {
	t.Helper()
	path := t.TempDir() + "/test.db"
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	if password != "" {
		if err := authsvc.SetPassword(sqlDB, password); err != nil {
			t.Fatalf("SetPassword: %v", err)
		}
	}

	hub := wsserver.NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)
	t.Cleanup(func() { close(stop) })

	srv := httptest.NewTLSServer(NewRouter(sqlDB, "", hub))
	t.Cleanup(srv.Close)
	return srv, sqlDB
}

func newCookieJar(t *testing.T, _ string) http.CookieJar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return jar
}

func postJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestLoginWrongPasswordThenSuccess(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	jar := newCookieJar(t, srv.URL)
	client.Jar = jar

	resp := postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "wrong"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for correct password, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err := client.Get(srv.URL + "/api/auth/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for authenticated status check, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestNotesRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	resp, err := srv.Client().Get(srv.URL + "/api/notes")
	if err != nil {
		t.Fatalf("GET notes: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestLogoutRevokesSession(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)

	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	resp, _ := client.Get(srv.URL + "/api/notes")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 while logged in, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	postJSON(t, client, srv.URL+"/api/auth/logout", map[string]string{}).Body.Close()

	resp, _ = client.Get(srv.URL + "/api/notes")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestLoginLockoutAfterRepeatedFailures(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)

	var last *http.Response
	for i := 0; i < 6; i++ {
		last = postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "wrong"})
		last.Body.Close()
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after repeated failures, got %d", last.StatusCode)
	}
}

func TestLoginWithNoPasswordConfigured(t *testing.T) {
	srv, _ := newTestServer(t, "")
	resp := postJSON(t, srv.Client(), srv.URL+"/api/auth/login", map[string]string{"password": "anything"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when no password configured, got %d", resp.StatusCode)
	}
}
