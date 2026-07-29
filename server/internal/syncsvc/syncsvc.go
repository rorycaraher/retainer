// Package syncsvc is the real, field-level-merge sync endpoint (see
// docs/adr/0001): the heart of the design. A mutation batch from one client
// is applied inside a single transaction, each field arbitrated against its
// stored HLC stamp, and the response returns authoritative state for
// anything the client hasn't seen — unifying push and pull into one call.
package syncsvc

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"retainer/server/internal/db"
	"retainer/server/internal/fracindex"
	"retainer/server/internal/hlc"
	"retainer/server/internal/models"
	"retainer/server/internal/searchsvc"
)

// Fields that affect a note's searchable text — everything else (pinned,
// position, ...) doesn't need a reindex.
var noteContentFields = map[string]bool{"title": true, "body": true, "kind": true}
var itemContentFields = map[string]bool{"text": true, "deleted": true}

// Mutation is one field-level edit, matching the wire protocol in the plan:
// {"entity": "note", "id": "...", "field": "title", "value": "...", "hlc": "..."}
type Mutation struct {
	Entity  string          `json:"entity"`
	ID      string          `json:"id,omitempty"`
	NoteID  string          `json:"noteId,omitempty"`
	LabelID string          `json:"labelId,omitempty"`
	Field   string          `json:"field"`
	Value   json.RawMessage `json:"value"`
	HLC     string          `json:"hlc"`
}

// NoteLabelChange is the wire shape for a note<->label attachment change.
type NoteLabelChange struct {
	NoteID    string `json:"noteId"`
	LabelID   string `json:"labelId"`
	Attached  bool   `json:"attached"`
	ServerSeq int64  `json:"serverSeq"`
}

// Changes is everything with server_seq > sinceSeq, returned to the client
// as authoritative state after a sync push (or on a plain catch-up pull).
type Changes struct {
	Notes      []*models.Note         `json:"notes"`
	Items      []models.ChecklistItem `json:"items"`
	NoteLabels []NoteLabelChange      `json:"noteLabels"`
}

var noteFieldColumns = map[string]string{
	"kind":            "kind",
	"title":           "title",
	"body":            "body",
	"color":           "color",
	"pinned":          "pinned",
	"archived":        "archived",
	"trashedAt":       "trashed_at",
	"position":        "position",
	"archivePosition": "archive_position",
}

var itemFieldColumns = map[string]string{
	"text":     "text",
	"checked":  "checked",
	"deleted":  "deleted",
	"position": "position",
}

// Apply applies a batch of mutations in a single transaction, then returns
// the new global server_seq plus everything changed since sinceSeq.
func Apply(sqlDB *sql.DB, sinceSeq int64, mutations []Mutation) (int64, Changes, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return 0, Changes{}, err
	}
	defer tx.Rollback()

	for i, m := range mutations {
		var err error
		switch m.Entity {
		case "note":
			err = applyNoteMutation(tx, m)
		case "item":
			err = applyItemMutation(tx, m)
		case "note_label":
			err = applyNoteLabelMutation(tx, m)
		default:
			err = fmt.Errorf("unknown mutation entity %q", m.Entity)
		}
		if err != nil {
			return 0, Changes{}, fmt.Errorf("mutation %d: %w", i, err)
		}
	}

	changes, err := loadChanges(tx, sinceSeq)
	if err != nil {
		return 0, Changes{}, err
	}
	seq, err := db.CurrentServerSeq(tx)
	if err != nil {
		return 0, Changes{}, err
	}

	if err := tx.Commit(); err != nil {
		return 0, Changes{}, err
	}
	return seq, changes, nil
}

func decodeString(raw json.RawMessage) (string, error) {
	var v string
	err := json.Unmarshal(raw, &v)
	return v, err
}

func decodeBool(raw json.RawMessage) (bool, error) {
	var v bool
	err := json.Unmarshal(raw, &v)
	return v, err
}

func decodeNullableInt64(raw json.RawMessage) (*int64, error) {
	var v *int64
	err := json.Unmarshal(raw, &v)
	return v, err
}

func applyNoteMutation(tx *sql.Tx, m Mutation) error {
	column, ok := noteFieldColumns[m.Field]
	if !ok {
		return fmt.Errorf("unknown note field %q", m.Field)
	}
	if m.ID == "" {
		return fmt.Errorf("note mutation missing id")
	}

	var fieldClocksJSON string
	err := tx.QueryRow(`SELECT field_clocks FROM notes WHERE id = ?`, m.ID).Scan(&fieldClocksJSON)
	now := time.Now().UnixMilli()

	if err == sql.ErrNoRows {
		seq, err := db.NextServerSeq(tx)
		if err != nil {
			return err
		}
		n := &models.Note{
			ID: m.ID,
			// Only used when the client's creation batch didn't include an
			// explicit "position" mutation (the normal client always sends
			// one, computed from its own list state) — a valid, if
			// arbitrarily placed, fracindex key rather than the old raw
			// timestamp placeholder, so later drag mutations interoperate
			// with it correctly.
			Kind:      "text",
			Color:     "default",
			Position:  fracindex.KeyBetween("", ""),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := applyFieldToNote(n, m.Field, m.Value); err != nil {
			return err
		}
		clocks := map[string]string{m.Field: m.HLC}
		clocksJSON, err := json.Marshal(clocks)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO notes (id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, field_clocks, server_seq, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, n.Kind, n.Title, n.Body, n.Color, n.Pinned, n.Archived, n.TrashedAt,
			n.Position, n.ArchivePosition, string(clocksJSON), seq, n.CreatedAt, n.UpdatedAt); err != nil {
			return err
		}
		return searchsvc.ReindexNote(tx, n.ID)
	}
	if err != nil {
		return err
	}

	clocks, err := unmarshalClocks(fieldClocksJSON)
	if err != nil {
		return err
	}
	if existing, has := clocks[m.Field]; has && hlc.Compare(m.HLC, existing) <= 0 {
		return nil // a stamp we've already applied, or an older concurrent edit — discard silently
	}

	seq, err := db.NextServerSeq(tx)
	if err != nil {
		return err
	}
	clocks[m.Field] = m.HLC
	clocksJSON, err := json.Marshal(clocks)
	if err != nil {
		return err
	}

	value, err := coercedValue(m.Field, m.Value)
	if err != nil {
		return err
	}
	//nolint:gosec // column is looked up from the fixed noteFieldColumns allowlist above, never client input
	query := fmt.Sprintf(`UPDATE notes SET %s = ?, field_clocks = ?, server_seq = ?, updated_at = ? WHERE id = ?`, column)
	if _, err = tx.Exec(query, value, string(clocksJSON), seq, now, m.ID); err != nil {
		return err
	}
	if noteContentFields[m.Field] {
		return searchsvc.ReindexNote(tx, m.ID)
	}
	return nil
}

func applyFieldToNote(n *models.Note, field string, raw json.RawMessage) error {
	switch field {
	case "kind":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		n.Kind = v
	case "title":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		n.Title = v
	case "body":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		n.Body = v
	case "color":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		n.Color = v
	case "pinned":
		v, err := decodeBool(raw)
		if err != nil {
			return err
		}
		n.Pinned = v
	case "archived":
		v, err := decodeBool(raw)
		if err != nil {
			return err
		}
		n.Archived = v
	case "trashedAt":
		v, err := decodeNullableInt64(raw)
		if err != nil {
			return err
		}
		n.TrashedAt = v
	case "position":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		n.Position = v
	case "archivePosition":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		n.ArchivePosition = v
	}
	return nil
}

func coercedValue(field string, raw json.RawMessage) (any, error) {
	switch field {
	case "pinned", "archived", "checked", "deleted":
		return decodeBool(raw)
	case "trashedAt":
		return decodeNullableInt64(raw)
	default:
		return decodeString(raw)
	}
}

func applyItemMutation(tx *sql.Tx, m Mutation) error {
	column, ok := itemFieldColumns[m.Field]
	if !ok {
		return fmt.Errorf("unknown item field %q", m.Field)
	}
	if m.ID == "" {
		return fmt.Errorf("item mutation missing id")
	}

	var fieldClocksJSON string
	err := tx.QueryRow(`SELECT field_clocks FROM checklist_items WHERE id = ?`, m.ID).Scan(&fieldClocksJSON)

	if err == sql.ErrNoRows {
		if m.NoteID == "" {
			return fmt.Errorf("item mutation for new item %q missing noteId", m.ID)
		}
		seq, err := db.NextServerSeq(tx)
		if err != nil {
			return err
		}
		it := models.ChecklistItem{ID: m.ID, NoteID: m.NoteID, Position: fracindex.KeyBetween("", "")}
		if err := applyFieldToItem(&it, m.Field, m.Value); err != nil {
			return err
		}
		clocks := map[string]string{m.Field: m.HLC}
		clocksJSON, err := json.Marshal(clocks)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO checklist_items (id, note_id, text, checked, deleted, position, field_clocks, server_seq)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			it.ID, it.NoteID, it.Text, it.Checked, it.Deleted, it.Position, string(clocksJSON), seq); err != nil {
			return err
		}
		return searchsvc.ReindexNote(tx, it.NoteID)
	}
	if err != nil {
		return err
	}

	clocks, err := unmarshalClocks(fieldClocksJSON)
	if err != nil {
		return err
	}
	if existing, has := clocks[m.Field]; has && hlc.Compare(m.HLC, existing) <= 0 {
		return nil
	}

	seq, err := db.NextServerSeq(tx)
	if err != nil {
		return err
	}
	clocks[m.Field] = m.HLC
	clocksJSON, err := json.Marshal(clocks)
	if err != nil {
		return err
	}
	value, err := coercedValue(m.Field, m.Value)
	if err != nil {
		return err
	}
	//nolint:gosec // column is looked up from the fixed itemFieldColumns allowlist above, never client input
	query := fmt.Sprintf(`UPDATE checklist_items SET %s = ?, field_clocks = ?, server_seq = ? WHERE id = ?`, column)
	if _, err = tx.Exec(query, value, string(clocksJSON), seq, m.ID); err != nil {
		return err
	}
	if itemContentFields[m.Field] {
		noteID := m.NoteID
		if noteID == "" {
			if err := tx.QueryRow(`SELECT note_id FROM checklist_items WHERE id = ?`, m.ID).Scan(&noteID); err != nil {
				return err
			}
		}
		return searchsvc.ReindexNote(tx, noteID)
	}
	return nil
}

func applyFieldToItem(it *models.ChecklistItem, field string, raw json.RawMessage) error {
	switch field {
	case "text":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		it.Text = v
	case "checked":
		v, err := decodeBool(raw)
		if err != nil {
			return err
		}
		it.Checked = v
	case "deleted":
		v, err := decodeBool(raw)
		if err != nil {
			return err
		}
		it.Deleted = v
	case "position":
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		it.Position = v
	}
	return nil
}

func applyNoteLabelMutation(tx *sql.Tx, m Mutation) error {
	if m.Field != "attached" {
		return fmt.Errorf("unknown note_label field %q", m.Field)
	}
	if m.NoteID == "" || m.LabelID == "" {
		return fmt.Errorf("note_label mutation missing noteId/labelId")
	}
	attached, err := decodeBool(m.Value)
	if err != nil {
		return err
	}

	var existingHLC string
	err = tx.QueryRow(`SELECT hlc FROM note_labels WHERE note_id = ? AND label_id = ?`, m.NoteID, m.LabelID).Scan(&existingHLC)

	if err == sql.ErrNoRows {
		seq, err := db.NextServerSeq(tx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO note_labels (note_id, label_id, attached, hlc, server_seq) VALUES (?, ?, ?, ?, ?)`,
			m.NoteID, m.LabelID, attached, m.HLC, seq)
		return err
	}
	if err != nil {
		return err
	}
	if hlc.Compare(m.HLC, existingHLC) <= 0 {
		return nil
	}

	seq, err := db.NextServerSeq(tx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE note_labels SET attached = ?, hlc = ?, server_seq = ? WHERE note_id = ? AND label_id = ?`,
		attached, m.HLC, seq, m.NoteID, m.LabelID)
	return err
}

func unmarshalClocks(raw string) (map[string]string, error) {
	clocks := map[string]string{}
	if raw == "" {
		return clocks, nil
	}
	if err := json.Unmarshal([]byte(raw), &clocks); err != nil {
		return nil, err
	}
	return clocks, nil
}

func loadChanges(tx *sql.Tx, sinceSeq int64) (Changes, error) {
	notes, err := loadChangedNotes(tx, sinceSeq)
	if err != nil {
		return Changes{}, err
	}
	items, err := loadChangedItems(tx, sinceSeq)
	if err != nil {
		return Changes{}, err
	}
	noteLabels, err := loadChangedNoteLabels(tx, sinceSeq)
	if err != nil {
		return Changes{}, err
	}
	return Changes{Notes: notes, Items: items, NoteLabels: noteLabels}, nil
}

func loadChangedNotes(tx *sql.Tx, sinceSeq int64) ([]*models.Note, error) {
	rows, err := tx.Query(`SELECT id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, server_seq, created_at, updated_at
		FROM notes WHERE server_seq > ? ORDER BY server_seq ASC`, sinceSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []*models.Note{}
	for rows.Next() {
		n := &models.Note{Items: []models.ChecklistItem{}, LabelIDs: []string{}}
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Color, &n.Pinned, &n.Archived, &n.TrashedAt,
			&n.Position, &n.ArchivePosition, &n.ServerSeq, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func loadChangedItems(tx *sql.Tx, sinceSeq int64) ([]models.ChecklistItem, error) {
	rows, err := tx.Query(`SELECT id, note_id, text, checked, deleted, position, server_seq
		FROM checklist_items WHERE server_seq > ? ORDER BY server_seq ASC`, sinceSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.ChecklistItem{}
	for rows.Next() {
		var it models.ChecklistItem
		if err := rows.Scan(&it.ID, &it.NoteID, &it.Text, &it.Checked, &it.Deleted, &it.Position, &it.ServerSeq); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func loadChangedNoteLabels(tx *sql.Tx, sinceSeq int64) ([]NoteLabelChange, error) {
	rows, err := tx.Query(`SELECT note_id, label_id, attached, server_seq FROM note_labels WHERE server_seq > ? ORDER BY server_seq ASC`, sinceSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	changes := []NoteLabelChange{}
	for rows.Next() {
		var c NoteLabelChange
		if err := rows.Scan(&c.NoteID, &c.LabelID, &c.Attached, &c.ServerSeq); err != nil {
			return nil, err
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}
