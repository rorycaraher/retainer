package notesvc

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"retainer/server/internal/db"
	"retainer/server/internal/syncsvc"
)

func jsonValue(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// A JSON-marshaled nil Go slice encodes as `null`, which breaks JS `for...of`
// on the client — List must always return a non-nil (possibly empty) slice.
func TestListOnEmptyDBReturnsEmptyNotNilSlice(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	list, err := List(sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil {
		t.Fatal("expected List to return a non-nil empty slice, got nil")
	}
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != "[]" {
		t.Fatalf("expected JSON [], got %s", b)
	}
}

func TestListAndGetPopulateLabelIDs(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	noteID := "note-1"
	_, _, err = syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note", ID: noteID, Field: "title", Value: jsonValue(t, "Labeled"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO labels (id, name, created_at) VALUES ('label-1', 'Work', 0)`); err != nil {
		t.Fatalf("seed label: %v", err)
	}
	_, _, err = syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note_label", NoteID: noteID, LabelID: "label-1", Field: "attached", Value: jsonValue(t, true), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("attach label: %v", err)
	}

	list, err := List(sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || len(list[0].LabelIDs) != 1 || list[0].LabelIDs[0] != "label-1" {
		t.Fatalf("expected List to include label-1, got %+v", list)
	}

	got, err := Get(sqlDB, noteID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.LabelIDs) != 1 || got.LabelIDs[0] != "label-1" {
		t.Fatalf("expected Get to include label-1, got %+v", got)
	}
}

func TestListAndGet(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	noteID := "note-1"
	_, _, err = syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note", ID: noteID, Field: "kind", Value: jsonValue(t, "checklist"), HLC: "00000000000000000001-0000000000-device-a"},
		{Entity: "note", ID: noteID, Field: "title", Value: jsonValue(t, "Groceries"), HLC: "00000000000000000001-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	list, err := List(sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != noteID {
		t.Fatalf("expected 1 note in list, got %+v", list)
	}

	got, err := Get(sqlDB, noteID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Groceries" || got.Kind != "checklist" {
		t.Fatalf("unexpected note: %+v", got)
	}

	itemID := "item-1"
	_, _, err = syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "item", ID: itemID, NoteID: noteID, Field: "text", Value: jsonValue(t, "Milk"), HLC: "00000000000000000002-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("Apply item: %v", err)
	}

	got, err = Get(sqlDB, noteID)
	if err != nil {
		t.Fatalf("Get after item: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Text != "Milk" {
		t.Fatalf("expected 1 item Milk, got %+v", got.Items)
	}
}

func TestListIncludesTrashedNotes(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()

	noteID := "note-1"
	_, _, err = syncsvc.Apply(sqlDB, 0, []syncsvc.Mutation{
		{Entity: "note", ID: noteID, Field: "title", Value: jsonValue(t, "Trash me"), HLC: "00000000000000000001-0000000000-device-a"},
		{Entity: "note", ID: noteID, Field: "trashedAt", Value: jsonValue(t, 1700000000000), HLC: "00000000000000000002-0000000000-device-a"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	list, err := List(sqlDB)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].TrashedAt == nil {
		t.Fatalf("expected trashed note included in List (client filters views locally), got %+v", list)
	}
}
