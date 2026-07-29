package syncsvc

import (
	"database/sql"
	"encoding/json"
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

func jv(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestCreateOnFirstMutation(t *testing.T) {
	sqlDB := openTestDB(t)

	seq, changes, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "Groceries"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if seq != 1 {
		t.Fatalf("expected serverSeq 1, got %d", seq)
	}
	if len(changes.Notes) != 1 {
		t.Fatalf("expected 1 changed note, got %d", len(changes.Notes))
	}
	n := changes.Notes[0]
	if n.ID != "note-1" || n.Title != "Groceries" || n.Kind != "text" {
		t.Fatalf("unexpected note: %+v", n)
	}
}

func TestConcurrentEditsToDifferentFieldsBothLand(t *testing.T) {
	sqlDB := openTestDB(t)

	// Two "devices" start from identical server state (an existing note),
	// each independently edits a different field, and pushes without
	// syncing between edits — both edits must survive the merge.
	_, _, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "Groceries"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("seed Apply: %v", err)
	}

	_, _, err = Apply(sqlDB, 1, []Mutation{
		{Entity: "note", ID: "note-1", Field: "pinned", Value: jv(t, true), HLC: "00000000000000000010-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("device A Apply: %v", err)
	}

	seq, changes, err := Apply(sqlDB, 1, []Mutation{
		{Entity: "item", ID: "item-1", NoteID: "note-1", Field: "text", Value: jv(t, "Milk"), HLC: "00000000000000000010-0000000000-device-b"},
	})
	if err != nil {
		t.Fatalf("device B Apply: %v", err)
	}
	if seq != 3 {
		t.Fatalf("expected serverSeq 3, got %d", seq)
	}

	if len(changes.Notes) != 1 || !changes.Notes[0].Pinned {
		t.Fatalf("expected pinned=true note in changes since sinceSeq=1, got %+v", changes.Notes)
	}
	if len(changes.Items) != 1 || changes.Items[0].Text != "Milk" {
		t.Fatalf("expected Milk item in changes, got %+v", changes.Items)
	}
}

func TestOlderConcurrentEditIsDiscarded(t *testing.T) {
	sqlDB := openTestDB(t)

	_, _, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "Original"), HLC: "00000000000000000005-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A mutation with an HLC older than the one already stored must be
	// discarded silently, not applied and not erroring.
	seq, _, err := Apply(sqlDB, 1, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "Stale"), HLC: "00000000000000000001-0000000000-device-b"},
	})
	if err != nil {
		t.Fatalf("Apply stale mutation: %v", err)
	}
	if seq != 1 {
		t.Fatalf("expected serverSeq to stay at 1 (no write happened), got %d", seq)
	}

	_, changes, err := Apply(sqlDB, 0, nil)
	if err != nil {
		t.Fatalf("Apply readback: %v", err)
	}
	if len(changes.Notes) != 1 || changes.Notes[0].Title != "Original" {
		t.Fatalf("expected title to remain Original, got %+v", changes.Notes)
	}
}

func TestNewerConcurrentEditWins(t *testing.T) {
	sqlDB := openTestDB(t)

	_, _, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "Original"), HLC: "00000000000000000005-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, _, err = Apply(sqlDB, 1, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "Newer"), HLC: "00000000000000000009-0000000000-device-b"},
	})
	if err != nil {
		t.Fatalf("Apply newer mutation: %v", err)
	}

	_, changes, err := Apply(sqlDB, 0, nil)
	if err != nil {
		t.Fatalf("Apply readback: %v", err)
	}
	if len(changes.Notes) != 1 || changes.Notes[0].Title != "Newer" {
		t.Fatalf("expected title to be Newer, got %+v", changes.Notes)
	}
}

func TestItemDeleteVsEditRevival(t *testing.T) {
	sqlDB := openTestDB(t)

	_, _, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "note", ID: "note-1", Field: "title", Value: jv(t, "List"), HLC: "00000000000000000001-0000000000-device-a"},
		{Entity: "item", ID: "item-1", NoteID: "note-1", Field: "text", Value: jv(t, "Milk"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Device A deletes the item at t=5.
	_, _, err = Apply(sqlDB, 2, []Mutation{
		{Entity: "item", ID: "item-1", Field: "deleted", Value: jv(t, true), HLC: "00000000000000000005-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Device B, offline since before the delete, edits the text at t=9 and
	// (per the client convention) bundles deleted:false at the same HLC —
	// a genuinely newer edit should revive the item.
	_, _, err = Apply(sqlDB, 2, []Mutation{
		{Entity: "item", ID: "item-1", Field: "text", Value: jv(t, "Oat milk"), HLC: "00000000000000000009-0000000000-device-b"},
		{Entity: "item", ID: "item-1", Field: "deleted", Value: jv(t, false), HLC: "00000000000000000009-0000000000-device-b"},
	})
	if err != nil {
		t.Fatalf("revive edit: %v", err)
	}

	_, changes, err := Apply(sqlDB, 0, nil)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if len(changes.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(changes.Items))
	}
	it := changes.Items[0]
	if it.Deleted || it.Text != "Oat milk" {
		t.Fatalf("expected item revived with new text, got %+v", it)
	}
}

func TestNoteLabelAttachDetachMerge(t *testing.T) {
	sqlDB := openTestDB(t)

	// note_labels has an FK to labels; insert one directly since labelsvc
	// (the REST CRUD layer) doesn't exist yet.
	if _, err := sqlDB.Exec(`INSERT INTO labels (id, name, created_at) VALUES ('label-1', 'Groceries', 0)`); err != nil {
		t.Fatalf("seed label: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO notes (id, kind, position, server_seq, created_at, updated_at) VALUES ('note-1', 'text', '0', 0, 0, 0)`); err != nil {
		t.Fatalf("seed note: %v", err)
	}

	_, _, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "note_label", NoteID: "note-1", LabelID: "label-1", Field: "attached", Value: jv(t, true), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	// A stale detach (older HLC) must not undo the attach.
	_, _, err = Apply(sqlDB, 1, []Mutation{
		{Entity: "note_label", NoteID: "note-1", LabelID: "label-1", Field: "attached", Value: jv(t, false), HLC: "00000000000000000000-0000000000-device-b"},
	})
	if err != nil {
		t.Fatalf("stale detach: %v", err)
	}

	_, changes, err := Apply(sqlDB, 0, nil)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if len(changes.NoteLabels) != 1 || !changes.NoteLabels[0].Attached {
		t.Fatalf("expected label to remain attached, got %+v", changes.NoteLabels)
	}

	// A newer detach must win.
	_, _, err = Apply(sqlDB, 2, []Mutation{
		{Entity: "note_label", NoteID: "note-1", LabelID: "label-1", Field: "attached", Value: jv(t, false), HLC: "00000000000000000005-0000000000-device-b"},
	})
	if err != nil {
		t.Fatalf("real detach: %v", err)
	}
	_, changes, err = Apply(sqlDB, 0, nil)
	if err != nil {
		t.Fatalf("readback 2: %v", err)
	}
	if len(changes.NoteLabels) != 1 || changes.NoteLabels[0].Attached {
		t.Fatalf("expected label detached, got %+v", changes.NoteLabels)
	}
}

func TestUnknownMutationEntityErrors(t *testing.T) {
	sqlDB := openTestDB(t)
	_, _, err := Apply(sqlDB, 0, []Mutation{
		{Entity: "bogus", ID: "x", Field: "title", Value: jv(t, "x"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err == nil {
		t.Fatal("expected error for unknown mutation entity")
	}
}
