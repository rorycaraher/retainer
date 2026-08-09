package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

func newAudioUploadRequest(t *testing.T, fields map[string]string, audioBytes []byte, audioContentType string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if audioBytes != nil {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="audio"; filename="recording"`}
		h["Content-Type"] = []string{audioContentType}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := part.Write(audioBytes); err != nil {
			t.Fatalf("write audio bytes: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return body, w.FormDataContentType()
}

func TestCreateAudioNoteRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	body, contentType := newAudioUploadRequest(t, map[string]string{
		"id": "note-1", "position": "a0", "durationMs": "1000",
	}, []byte("fake-audio"), "audio/webm")

	resp, err := srv.Client().Post(srv.URL+"/api/notes/audio", contentType, body)
	if err != nil {
		t.Fatalf("POST /api/notes/audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", resp.StatusCode)
	}
}

func TestCreateAudioNoteThenFetchesRecording(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)
	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	audioBytes := []byte("fake-opus-bytes")
	body, contentType := newAudioUploadRequest(t, map[string]string{
		"id": "note-1", "position": "a0", "durationMs": "4200", "title": "Voice note",
	}, audioBytes, "audio/webm")

	resp, err := client.Post(srv.URL+"/api/notes/audio", contentType, body)
	if err != nil {
		t.Fatalf("POST /api/notes/audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}

	var note struct {
		ID              string `json:"id"`
		Kind            string `json:"kind"`
		Title           string `json:"title"`
		AudioMimeType   string `json:"audioMimeType"`
		AudioDurationMs int64  `json:"audioDurationMs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if note.Kind != "audio" || note.Title != "Voice note" || note.AudioMimeType != "audio/webm" || note.AudioDurationMs != 4200 {
		t.Fatalf("unexpected note: %+v", note)
	}

	// List must never leak the raw blob into the general note payload.
	listResp, err := client.Get(srv.URL + "/api/notes")
	if err != nil {
		t.Fatalf("GET /api/notes: %v", err)
	}
	defer listResp.Body.Close()
	listBody, err := io.ReadAll(listResp.Body)
	if err != nil {
		t.Fatalf("read list body: %v", err)
	}
	if bytes.Contains(listBody, audioBytes) {
		t.Fatal("expected /api/notes list to exclude raw audio bytes")
	}

	audioResp, err := client.Get(srv.URL + "/api/notes/note-1/audio")
	if err != nil {
		t.Fatalf("GET audio: %v", err)
	}
	defer audioResp.Body.Close()
	if ct := audioResp.Header.Get("Content-Type"); ct != "audio/webm" {
		t.Fatalf("expected audio/webm content type, got %q", ct)
	}
	got, err := io.ReadAll(audioResp.Body)
	if err != nil {
		t.Fatalf("read audio body: %v", err)
	}
	if !bytes.Equal(got, audioBytes) {
		t.Fatalf("expected fetched audio bytes to match upload, got %q", got)
	}
}

func TestCreateAudioNoteRejectsOverLongRecording(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)
	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	body, contentType := newAudioUploadRequest(t, map[string]string{
		"id": "note-1", "position": "a0", "durationMs": "600000", // 10 minutes, over the 5 minute cap
	}, []byte("fake-audio"), "audio/webm")

	resp, err := client.Post(srv.URL+"/api/notes/audio", contentType, body)
	if err != nil {
		t.Fatalf("POST /api/notes/audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for over-cap duration, got %d", resp.StatusCode)
	}
}

func TestGetAudioNotFoundForMissingOrNonAudioNote(t *testing.T) {
	srv, _ := newTestServer(t, "hunter2")
	client := srv.Client()
	client.Jar = newCookieJar(t, srv.URL)
	postJSON(t, client, srv.URL+"/api/auth/login", map[string]string{"password": "hunter2"}).Body.Close()

	resp, err := client.Get(srv.URL + "/api/notes/does-not-exist/audio")
	if err != nil {
		t.Fatalf("GET audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for missing note, got %d", resp.StatusCode)
	}
}
