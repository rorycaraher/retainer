package purge

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"retainer/server/internal/db"
	"retainer/server/internal/syncsvc"
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

func jv(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func trashNote(t *testing.T, sqlDB *sql.DB, id string, trashedAtMs int64) {
	t.Helper()
	_, _, err := syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note", ID: id, Field: "title", Value: jv(t, "note "+id), HLC: "00000000000000000001-0000000000-device-a"},
		{Entity: "note", ID: id, Field: "trashedAt", Value: jv(t, trashedAtMs), HLC: "00000000000000000002-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("trashNote(%s): %v", id, err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO checklist_items (id, note_id, position, server_seq) VALUES (?, ?, '0', 0)`, id+"-item", id); err != nil {
		t.Fatalf("seed item for %s: %v", id, err)
	}
}

func noteExists(t *testing.T, sqlDB *sql.DB, id string) bool {
	t.Helper()
	var exists int
	err := sqlDB.QueryRow(`SELECT 1 FROM notes WHERE id = ?`, id).Scan(&exists)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("query note %s: %v", id, err)
	}
	return true
}

func itemsExist(t *testing.T, sqlDB *sql.DB, noteID string) bool {
	t.Helper()
	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM checklist_items WHERE note_id = ?`, noteID).Scan(&count); err != nil {
		t.Fatalf("count items for %s: %v", noteID, err)
	}
	return count > 0
}

func TestSweepRemovesOnlyNotesPastRetention(t *testing.T) {
	sqlDB := openTestDB(t)
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	trashNote(t, sqlDB, "old", now.Add(-8*24*time.Hour).UnixMilli())    // past 7d, should be purged
	trashNote(t, sqlDB, "recent", now.Add(-1*24*time.Hour).UnixMilli()) // within 7d, should survive
	trashNote(t, sqlDB, "exactly-7d", now.Add(-7*24*time.Hour).UnixMilli())

	n, err := Sweep(sqlDB, now)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 note purged, got %d", n)
	}
	if noteExists(t, sqlDB, "old") {
		t.Fatal("expected 'old' note to be purged")
	}
	if itemsExist(t, sqlDB, "old") {
		t.Fatal("expected 'old' note's items to be purged")
	}
	if !noteExists(t, sqlDB, "recent") {
		t.Fatal("expected 'recent' note to survive")
	}
	if !noteExists(t, sqlDB, "exactly-7d") {
		t.Fatal("expected note trashed exactly 7d ago to still survive (cutoff is strictly older)")
	}
}

func TestSweepIgnoresUntrashedNotes(t *testing.T) {
	sqlDB := openTestDB(t)
	_, _, err := syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note", ID: "alive", Field: "title", Value: jv(t, "still here"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	n, err := Sweep(sqlDB, time.Now().Add(365*24*time.Hour))
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 purged, got %d", n)
	}
	if !noteExists(t, sqlDB, "alive") {
		t.Fatal("expected untrashed note to survive regardless of how far in the future Sweep runs")
	}
}

func TestPurgeOneDeletesImmediatelyRegardlessOfTrashedAt(t *testing.T) {
	sqlDB := openTestDB(t)
	_, _, err := syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note", ID: "n1", Field: "title", Value: jv(t, "delete me now"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO checklist_items (id, note_id, position, server_seq) VALUES ('i1', 'n1', '0', 0)`); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	if err := PurgeOne(sqlDB, "n1"); err != nil {
		t.Fatalf("PurgeOne: %v", err)
	}
	if noteExists(t, sqlDB, "n1") {
		t.Fatal("expected note purged immediately even though it was never trashed")
	}
	if itemsExist(t, sqlDB, "n1") {
		t.Fatal("expected items purged too")
	}
}
