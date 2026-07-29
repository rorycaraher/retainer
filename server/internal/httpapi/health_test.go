package httpapi

import (
	"net/http"
	"testing"
)

func TestHealthDoesNotRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	resp, err := srv.Client().Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
