package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestLabelsRequireAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	resp, err := srv.Client().Get(srv.URL + "/api/labels")
	if err != nil {
		t.Fatalf("GET /api/labels: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}
}

func loggedInClient(t *testing.T, srv interface{ Client() *http.Client }, srvURL, password string) *http.Client {
	t.Helper()
	client := srv.Client()
	client.Jar = newCookieJar(t, srvURL)
	postJSON(t, client, srvURL+"/api/auth/login", map[string]string{"password": password}).Body.Close()
	return client
}

func TestLabelCRUDLifecycle(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := loggedInClient(t, srv, srv.URL, "hunter2")

	createResp := postJSON(t, client, srv.URL+"/api/labels", map[string]string{"name": "Work"})
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createResp.StatusCode)
	}
	var created struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Name != "Work" || created.ID == "" {
		t.Fatalf("unexpected created label: %+v", created)
	}

	listResp, err := client.Get(srv.URL + "/api/labels")
	if err != nil {
		t.Fatalf("GET /api/labels: %v", err)
	}
	var listOut struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listOut); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	listResp.Body.Close()
	if len(listOut.Labels) != 1 || listOut.Labels[0].ID != created.ID {
		t.Fatalf("expected 1 label, got %+v", listOut.Labels)
	}

	renameBody, err := json.Marshal(map[string]string{"name": "Job"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	renameReq, _ := http.NewRequest(http.MethodPatch, srv.URL+"/api/labels/"+created.ID, bytes.NewReader(renameBody))
	renameReq.Header.Set("Content-Type", "application/json")
	renameResp, err := client.Do(renameReq)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer renameResp.Body.Close()
	if renameResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", renameResp.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/labels/"+created.ID, nil)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	finalList, err := client.Get(srv.URL + "/api/labels")
	if err != nil {
		t.Fatalf("GET /api/labels: %v", err)
	}
	defer finalList.Body.Close()
	var finalOut struct {
		Labels []any `json:"labels"`
	}
	json.NewDecoder(finalList.Body).Decode(&finalOut)
	if len(finalOut.Labels) != 0 {
		t.Fatalf("expected 0 labels after delete, got %d", len(finalOut.Labels))
	}
}

func TestCreateLabelDuplicateNameConflicts(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := loggedInClient(t, srv, srv.URL, "hunter2")

	postJSON(t, client, srv.URL+"/api/labels", map[string]string{"name": "Work"}).Body.Close()
	resp := postJSON(t, client, srv.URL+"/api/labels", map[string]string{"name": "Work"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}
