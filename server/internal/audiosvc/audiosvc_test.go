package audiosvc

import (
	"database/sql"
	"path/filepath"
	"testing"

	"retainer/server/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB
}

func TestCreateAndFetchAudio(t *testing.T) {
	sqlDB := openTestDB(t)
	blob := []byte("fake-recording-bytes")

	note, err := Create(sqlDB, "note-1", "Voice note", "a0", "audio/webm", 4200, blob)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if note.Kind != "audio" || note.AudioMimeType != "audio/webm" || note.AudioDurationMs != 4200 {
		t.Fatalf("unexpected note: %+v", note)
	}

	gotBlob, mimeType, err := Audio(sqlDB, "note-1")
	if err != nil {
		t.Fatalf("Audio: %v", err)
	}
	if string(gotBlob) != string(blob) || mimeType != "audio/webm" {
		t.Fatalf("expected matching blob/mimeType, got %q %q", gotBlob, mimeType)
	}
}

func TestAudioNotFoundForMissingNote(t *testing.T) {
	sqlDB := openTestDB(t)
	_, _, err := Audio(sqlDB, "does-not-exist")
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestAudioNotFoundForNonAudioNote(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := sqlDB.Exec(`INSERT INTO notes (id, kind, position, field_clocks, server_seq, created_at, updated_at)
		VALUES ('note-1', 'text', 'a0', '{}', 1, 0, 0)`); err != nil {
		t.Fatalf("seed text note: %v", err)
	}
	_, _, err := Audio(sqlDB, "note-1")
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows for non-audio note, got %v", err)
	}
}
