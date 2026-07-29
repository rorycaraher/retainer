package db

import (
	"path/filepath"
	"testing"
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
	if count != 1 {
		t.Fatalf("expected 1 applied migration, got %d", count)
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
	if count != 1 {
		t.Fatalf("expected migrations to be applied exactly once, got %d rows", count)
	}
}
