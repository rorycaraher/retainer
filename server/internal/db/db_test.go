package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"retainer/server/migrations"
)

func TestOpenAppliesMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	sqlDB, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	var count int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", count)
	}

	// notes table should exist and be empty.
	var noteCount int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM notes`).Scan(&noteCount); err != nil {
		t.Fatalf("query notes: %v", err)
	}
	if noteCount != 0 {
		t.Fatalf("expected empty notes table, got %d rows", noteCount)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	var count int
	if err := db2.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected migrations to be applied exactly once, got %d rows", count)
	}
}

// TestMigrationOntoExistingDataWithForeignKeys guards against a real incident:
// 002_audio_notes.sql rebuilds the notes table (DROP + recreate, since SQLite
// can't widen a CHECK constraint in place) to add the "audio" kind. Every
// other test here opens a brand-new file, so migrations 001 and 002 always
// ran together against an empty database — that never exercised what happens
// upgrading a database that already has 001 applied AND real rows with a live
// foreign key into notes (checklist_items.note_id), which is exactly what a
// production deploy does. That combination failed in production: SQLite's
// deferred-FK bookkeeping doesn't reconcile a table drop-and-recreate under
// the same name, so the migration transaction refused to commit. This test
// reproduces that starting state directly (apply 001 by hand, seed rows,
// THEN run the real migrate-forward path) rather than starting from empty.
func TestMigrationOntoExistingDataWithForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	seedSQL, err := migrations.FS.ReadFile("001_init.sql")
	if err != nil {
		t.Fatalf("read 001_init.sql: %v", err)
	}
	seedDB, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path))
	if err != nil {
		t.Fatalf("open for seeding: %v", err)
	}
	if _, err := seedDB.Exec(string(seedSQL)); err != nil {
		t.Fatalf("apply 001_init.sql: %v", err)
	}
	if _, err := seedDB.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (1, 0)`); err != nil {
		t.Fatalf("record migration 1: %v", err)
	}
	if _, err := seedDB.Exec(`INSERT INTO notes (id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, field_clocks, server_seq, created_at, updated_at)
		VALUES ('note-a', 'checklist', 'Groceries', '', 'default', 0, 0, NULL, 'm', '', '{}', 1, 1000, 1000)`); err != nil {
		t.Fatalf("seed note: %v", err)
	}
	if _, err := seedDB.Exec(`INSERT INTO checklist_items (id, note_id, text, checked, deleted, position, field_clocks, server_seq)
		VALUES ('item-a', 'note-a', 'Milk', 0, 0, 'm', '{}', 2)`); err != nil {
		t.Fatalf("seed checklist item: %v", err)
	}
	if err := seedDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	sqlDB, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migrate forward onto existing data): %v", err)
	}
	defer sqlDB.Close()

	var title string
	if err := sqlDB.QueryRow(`SELECT title FROM notes WHERE id = 'note-a'`).Scan(&title); err != nil {
		t.Fatalf("query migrated note: %v", err)
	}
	if title != "Groceries" {
		t.Fatalf("expected note title preserved, got %q", title)
	}
	var itemText string
	if err := sqlDB.QueryRow(`SELECT text FROM checklist_items WHERE id = 'item-a'`).Scan(&itemText); err != nil {
		t.Fatalf("query migrated checklist item: %v", err)
	}
	if itemText != "Milk" {
		t.Fatalf("expected checklist item preserved, got %q", itemText)
	}

	// New "audio" kind must actually be usable post-migration.
	if _, err := sqlDB.Exec(`INSERT INTO notes (id, kind, position, field_clocks, server_seq, created_at, updated_at, audio_mime_type, audio_duration_ms)
		VALUES ('note-b', 'audio', 'n', '{}', 3, 2000, 2000, 'audio/webm', 1500)`); err != nil {
		t.Fatalf("insert audio note after migration: %v", err)
	}
}
