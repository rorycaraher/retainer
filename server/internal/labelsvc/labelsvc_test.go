package labelsvc

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

func TestCreateListRenameDelete(t *testing.T) {
	sqlDB := openTestDB(t)

	l, err := Create(sqlDB, "Groceries", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if l.ID == "" || l.Name != "Groceries" {
		t.Fatalf("unexpected label: %+v", l)
	}

	list, err := List(sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != l.ID {
		t.Fatalf("expected 1 label, got %+v", list)
	}

	renamed, err := Rename(sqlDB, l.ID, "Shopping")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Name != "Shopping" {
		t.Fatalf("expected renamed label, got %+v", renamed)
	}

	if err := Delete(sqlDB, l.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, err = List(sqlDB)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 labels after delete, got %+v", list)
	}
}

func TestCreateRejectsEmptyName(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := Create(sqlDB, "   ", 1000); err == nil {
		t.Fatal("expected error for empty/whitespace-only name")
	}
}

func TestCreateRejectsDuplicateName(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := Create(sqlDB, "Work", 1000); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := Create(sqlDB, "Work", 2000); err != ErrNameTaken {
		t.Fatalf("expected ErrNameTaken, got %v", err)
	}
}

func TestRenameRejectsDuplicateName(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := Create(sqlDB, "Work", 1000); err != nil {
		t.Fatalf("Create Work: %v", err)
	}
	l2, err := Create(sqlDB, "Home", 1000)
	if err != nil {
		t.Fatalf("Create Home: %v", err)
	}
	if _, err := Rename(sqlDB, l2.ID, "Work"); err != ErrNameTaken {
		t.Fatalf("expected ErrNameTaken, got %v", err)
	}
}

func TestRenameNonexistentReturnsNotFound(t *testing.T) {
	sqlDB := openTestDB(t)
	if _, err := Rename(sqlDB, "bogus-id", "New Name"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteStripsLabelFromAllNotes(t *testing.T) {
	sqlDB := openTestDB(t)
	l, err := Create(sqlDB, "Work", 1000)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Attach the label to two notes directly (attach/detach mutation logic
	// itself is syncsvc's job and is tested there).
	if _, err := sqlDB.Exec(`INSERT INTO notes (id, kind, position, server_seq, created_at, updated_at) VALUES ('n1', 'text', '0', 0, 0, 0)`); err != nil {
		t.Fatalf("seed note n1: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO notes (id, kind, position, server_seq, created_at, updated_at) VALUES ('n2', 'text', '1', 0, 0, 0)`); err != nil {
		t.Fatalf("seed note n2: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO note_labels (note_id, label_id, attached, hlc, server_seq) VALUES ('n1', ?, 1, 'x', 0)`, l.ID); err != nil {
		t.Fatalf("attach to n1: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO note_labels (note_id, label_id, attached, hlc, server_seq) VALUES ('n2', ?, 1, 'x', 0)`, l.ID); err != nil {
		t.Fatalf("attach to n2: %v", err)
	}

	if err := Delete(sqlDB, l.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM note_labels WHERE label_id = ?`, l.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected label stripped from all notes, found %d attachments remaining", count)
	}
}

func TestDeleteNonexistentReturnsNotFound(t *testing.T) {
	sqlDB := openTestDB(t)
	if err := Delete(sqlDB, "bogus-id"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
