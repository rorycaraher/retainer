package httpapi

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	return u
}

func TestWSRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	wsURL := "wss" + strings.TrimPrefix(srv.URL, "https") + "/ws"

	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	_, resp, err := dialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial to fail without a session cookie")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 handshake response, got %+v", resp)
	}
}

func TestWSReceivesChangedEventAfterSync(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	jar := newCookieJar(t, srv.URL)
	client.Jar = jar
	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	wsURL := "wss" + strings.TrimPrefix(srv.URL, "https") + "/ws"
	header := http.Header{}
	for _, c := range jar.Cookies(mustParseURL(t, srv.URL)) {
		header.Add("Cookie", c.String())
	}

	dialer := websocket.Dialer{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	postJSON(t, client, srv.URL+"/api/sync", map[string]any{
		"sinceSeq": 0,
		"mutations": []map[string]any{
			{"entity": "note", "id": "note-1", "field": "title", "value": "Groceries", "hlc": "00000000000000000001-0000000000-device-a"},
		},
	}).Body.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if !strings.Contains(string(msg), `"type":"changed"`) {
		t.Fatalf("expected a changed event, got %s", msg)
	}
}
