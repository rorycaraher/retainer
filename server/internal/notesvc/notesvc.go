// Package notesvc provides read access to Notes and Checklist Items for the
// initial-load snapshot endpoint. All writes go through syncsvc, the
// field-level-merge sync engine.
package notesvc

import (
	"database/sql"

	"retainer/server/internal/models"
)

// List returns every Note (including trashed and archived ones — the client
// derives its Main/Archive/Trash views from one local copy of everything,
// the same way it reconciles incremental /api/sync responses), with their
// Checklist Items.
func List(db *sql.DB) ([]*models.Note, error) {
	rows, err := db.Query(`SELECT id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, server_seq, created_at, updated_at
		FROM notes ORDER BY position DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []*models.Note{}
	for rows.Next() {
		n := &models.Note{}
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Color, &n.Pinned, &n.Archived, &n.TrashedAt,
			&n.Position, &n.ArchivePosition, &n.ServerSeq, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Items = []models.ChecklistItem{}
		n.LabelIDs = []string{}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, n := range notes {
		items, err := itemsForNote(db, n.ID)
		if err != nil {
			return nil, err
		}
		n.Items = items
		labelIDs, err := labelIDsForNote(db, n.ID)
		if err != nil {
			return nil, err
		}
		n.LabelIDs = labelIDs
	}
	return notes, nil
}

func labelIDsForNote(db *sql.DB, noteID string) ([]string, error) {
	rows, err := db.Query(`SELECT label_id FROM note_labels WHERE note_id = ? AND attached = 1`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func itemsForNote(db *sql.DB, noteID string) ([]models.ChecklistItem, error) {
	rows, err := db.Query(`SELECT id, note_id, text, checked, position, server_seq
		FROM checklist_items WHERE note_id = ? AND deleted = 0 ORDER BY position ASC`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.ChecklistItem{}
	for rows.Next() {
		var it models.ChecklistItem
		if err := rows.Scan(&it.ID, &it.NoteID, &it.Text, &it.Checked, &it.Position, &it.ServerSeq); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// Get returns a single Note by id, or sql.ErrNoRows if it doesn't exist.
func Get(db *sql.DB, id string) (*models.Note, error) {
	n := &models.Note{}
	err := db.QueryRow(`SELECT id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, server_seq, created_at, updated_at
		FROM notes WHERE id = ?`, id).
		Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.Color, &n.Pinned, &n.Archived, &n.TrashedAt,
			&n.Position, &n.ArchivePosition, &n.ServerSeq, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	items, err := itemsForNote(db, n.ID)
	if err != nil {
		return nil, err
	}
	n.Items = items
	labelIDs, err := labelIDsForNote(db, n.ID)
	if err != nil {
		return nil, err
	}
	n.LabelIDs = labelIDs
	return n, nil
}
