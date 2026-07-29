package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"retainer/server/internal/syncsvc"
)

func TestPurgeRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/notes/note-1/purge", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE purge: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}
}

func TestPurgeDeletesNoteImmediately(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)
	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	postJSON(t, client, srv.URL+"/api/sync", map[string]any{
		"sinceSeq": 0,
		"mutations": []syncsvc.Mutation{
			{Entity: "note", ID: "note-1", Field: "title", Value: rawJSON(t, "Gone soon"), HLC: "00000000000000000001-0000000000-device-a"},
		},
	}).Body.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/notes/note-1/purge", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE purge: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	listResp, err := client.Get(srv.URL + "/api/notes")
	if err != nil {
		t.Fatalf("GET /api/notes: %v", err)
	}
	defer listResp.Body.Close()
	var out struct {
		Notes []struct {
			ID string `json:"id"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, n := range out.Notes {
		if n.ID == "note-1" {
			t.Fatal("expected note-1 to be gone after purge")
		}
	}
}
