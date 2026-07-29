package searchsvc

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

type noteOpts struct {
	title, body, kind string
	trashed, archived bool
	items             []string // checklist item text, only used when kind == "checklist"
}

// seedNote inserts a note row (and optional checklist items) directly via
// SQL, then reindexes it — searchsvc's own unit, without going through
// syncsvc (which imports this package, so importing it back here in tests
// would be a cycle).
func seedNote(t *testing.T, sqlDB *sql.DB, id string, o noteOpts) {
	t.Helper()
	if o.kind == "" {
		o.kind = "text"
	}
	var trashedAt any
	if o.trashed {
		trashedAt = 123
	}
	_, err := sqlDB.Exec(`INSERT INTO notes (id, kind, title, body, archived, trashed_at, position, server_seq, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '0', 0, 0, 0)`, id, o.kind, o.title, o.body, o.archived, trashedAt)
	if err != nil {
		t.Fatalf("seed note %s: %v", id, err)
	}
	for i, text := range o.items {
		_, err := sqlDB.Exec(`INSERT INTO checklist_items (id, note_id, text, position, server_seq) VALUES (?, ?, ?, ?, 0)`,
			id+"-item-"+string(rune('a'+i)), id, text, string(rune('a'+i)))
		if err != nil {
			t.Fatalf("seed item for %s: %v", id, err)
		}
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := ReindexNote(tx, id); err != nil {
		tx.Rollback()
		t.Fatalf("ReindexNote(%s): %v", id, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestSearchFindsNoteByTitleAndBody(t *testing.T) {
	sqlDB := openTestDB(t)
	seedNote(t, sqlDB, "n1", noteOpts{title: "Grocery list", body: "buy milk and eggs"})

	for _, q := range []string{"grocery", "milk", "Grocery List", "eggs"} {
		ids, err := Search(sqlDB, q)
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		if len(ids) != 1 || ids[0] != "n1" {
			t.Fatalf("Search(%q): expected [n1], got %v", q, ids)
		}
	}

	ids, err := Search(sqlDB, "nonexistentword")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no matches, got %v", ids)
	}
}

func TestSearchFindsChecklistItemText(t *testing.T) {
	sqlDB := openTestDB(t)
	seedNote(t, sqlDB, "n1", noteOpts{title: "Shopping", kind: "checklist", items: []string{"sourdough bread"}})

	ids, err := Search(sqlDB, "sourdough")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(ids) != 1 || ids[0] != "n1" {
		t.Fatalf("expected [n1], got %v", ids)
	}
}

func TestSearchExcludesTrashedAndArchivedNotes(t *testing.T) {
	sqlDB := openTestDB(t)
	seedNote(t, sqlDB, "trashed", noteOpts{title: "unicornfish", trashed: true})
	seedNote(t, sqlDB, "archived", noteOpts{title: "unicornfish", archived: true})
	seedNote(t, sqlDB, "visible", noteOpts{title: "unicornfish"})

	ids, err := Search(sqlDB, "unicornfish")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(ids) != 1 || ids[0] != "visible" {
		t.Fatalf("expected only [visible], got %v", ids)
	}
}

func TestReindexReflectsEditsAndDeletes(t *testing.T) {
	sqlDB := openTestDB(t)
	seedNote(t, sqlDB, "n1", noteOpts{title: "original title"})
	if ids, _ := Search(sqlDB, "original"); len(ids) != 1 {
		t.Fatalf("expected to find original title, got %v", ids)
	}

	if _, err := sqlDB.Exec(`UPDATE notes SET title = 'updated title' WHERE id = 'n1'`); err != nil {
		t.Fatalf("update: %v", err)
	}
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := ReindexNote(tx, "n1"); err != nil {
		t.Fatalf("ReindexNote: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if ids, _ := Search(sqlDB, "original"); len(ids) != 0 {
		t.Fatalf("expected stale title no longer indexed, got %v", ids)
	}
	if ids, _ := Search(sqlDB, "updated"); len(ids) != 1 {
		t.Fatalf("expected updated title indexed, got %v", ids)
	}
}

func TestReindexOfMissingNoteRemovesStaleFTSRow(t *testing.T) {
	sqlDB := openTestDB(t)
	seedNote(t, sqlDB, "n1", noteOpts{title: "will be purged"})
	if ids, _ := Search(sqlDB, "purged"); len(ids) != 1 {
		t.Fatalf("expected indexed before purge, got %v", ids)
	}

	if _, err := sqlDB.Exec(`DELETE FROM notes WHERE id = 'n1'`); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := ReindexNote(tx, "n1"); err != nil {
		t.Fatalf("ReindexNote on missing note: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM notes_fts WHERE note_id = 'n1'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected stale FTS row removed, found %d", count)
	}
}

func TestSearchQueryEscaping(t *testing.T) {
	sqlDB := openTestDB(t)
	seedNote(t, sqlDB, "n1", noteOpts{title: "AND OR test"})

	// These would be malformed/dangerous FTS5 syntax if passed through
	// unescaped — Search must not error and must treat them as literal text.
	malicious := []string{
		`"unterminated quote`,
		`AND OR NOT`,
		`*`,
		`title:"injection"`,
		`-excluded`,
		``,
		`   `,
	}
	for _, q := range malicious {
		if _, err := Search(sqlDB, q); err != nil {
			t.Fatalf("Search(%q) errored: %v", q, err)
		}
	}
}

func TestSearchEmptyQueryReturnsEmpty(t *testing.T) {
	sqlDB := openTestDB(t)
	ids, err := Search(sqlDB, "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no results for empty query, got %v", ids)
	}
}
