package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"retainer/server/internal/syncsvc"
)

func TestSearchRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	resp, err := srv.Client().Get(srv.URL + "/api/notes/search?q=milk")
	if err != nil {
		t.Fatalf("GET search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}
}

func TestSearchEndToEnd(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := loggedInClient(t, srv, srv.URL, "hunter2")

	postJSON(t, client, srv.URL+"/api/sync", map[string]any{
		"sinceSeq": 0,
		"mutations": []syncsvc.Mutation{
			{Entity: "note", ID: "n1", Field: "title", Value: rawJSON(t, "Grocery run"), HLC: "00000000000000000001-0000000000-device-a"},
			{Entity: "note", ID: "n1", Field: "body", Value: rawJSON(t, "need milk"), HLC: "00000000000000000002-0000000000-device-a"},
		},
	}).Body.Close()

	resp, err := client.Get(srv.URL + "/api/notes/search?q=milk")
	if err != nil {
		t.Fatalf("GET search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		Notes []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Notes) != 1 || out.Notes[0].ID != "n1" {
		t.Fatalf("expected [n1], got %+v", out.Notes)
	}
}
