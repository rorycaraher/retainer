package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"retainer/server/internal/syncsvc"
)

func TestSyncRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	resp, err := srv.Client().Post(srv.URL+"/api/sync", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/sync: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}
}

func TestSyncEndpointTwoDeviceMerge(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)
	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	seedResp := postJSON(t, client, srv.URL+"/api/sync", map[string]any{
		"sinceSeq": 0,
		"mutations": []syncsvc.Mutation{
			{Entity: "note", ID: "note-1", Field: "title", Value: rawJSON(t, "Groceries"), HLC: "00000000000000000001-0000000000-device-a"},
		},
	})
	var seedOut struct {
		ServerSeq int64 `json:"serverSeq"`
	}
	if err := json.NewDecoder(seedResp.Body).Decode(&seedOut); err != nil {
		t.Fatalf("decode seed response: %v", err)
	}
	seedResp.Body.Close()

	// Device A pins the note; device B (same session for this test, since
	// auth is per-user not per-device) edits a different field — both
	// pushed without syncing between edits.
	postJSON(t, client, srv.URL+"/api/sync", map[string]any{
		"sinceSeq": seedOut.ServerSeq,
		"mutations": []syncsvc.Mutation{
			{Entity: "note", ID: "note-1", Field: "pinned", Value: rawJSON(t, true), HLC: "00000000000000000010-0000000000-device-a"},
		},
	}).Body.Close()

	finalResp := postJSON(t, client, srv.URL+"/api/sync", map[string]any{
		"sinceSeq": seedOut.ServerSeq,
		"mutations": []syncsvc.Mutation{
			{Entity: "item", ID: "item-1", NoteID: "note-1", Field: "text", Value: rawJSON(t, "Milk"), HLC: "00000000000000000010-0000000000-device-b"},
		},
	})
	defer finalResp.Body.Close()
	if finalResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", finalResp.StatusCode)
	}

	var out struct {
		ServerSeq int64 `json:"serverSeq"`
		Changes   struct {
			Notes []struct {
				Title  string `json:"title"`
				Pinned bool   `json:"pinned"`
			} `json:"notes"`
			Items []struct {
				Text string `json:"text"`
			} `json:"items"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(finalResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(out.Changes.Notes) != 1 || !out.Changes.Notes[0].Pinned {
		t.Fatalf("expected pinned note in changes, got %+v", out.Changes.Notes)
	}
	if len(out.Changes.Items) != 1 || out.Changes.Items[0].Text != "Milk" {
		t.Fatalf("expected Milk item in changes, got %+v", out.Changes.Items)
	}
}

func rawJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
