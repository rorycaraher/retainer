// Package audiosvc creates and serves Audio Notes. Unlike Text Notes and
// Checklists, an Audio Note's recording is immutable from the moment it's
// saved (docs/adr/0005), so it's written once in a single transaction here
// rather than through syncsvc's field-level-merge mutation batch — there's
// no field to arbitrate via HLC for content that never changes again.
package audiosvc

import (
	"database/sql"
	"time"

	"retainer/server/internal/db"
	"retainer/server/internal/models"
	"retainer/server/internal/searchsvc"
)

// MaxDurationMs is the hard cap on a recording's length (CONTEXT.md: Audio
// Note is "capped at 5 minutes"), with a couple seconds of slack for
// encoding/rounding beyond the client's own 5:00 auto-stop.
const MaxDurationMs = 5*60*1000 + 2000

// Create inserts a new Audio Note as one atomic write and returns it
// (without the blob itself — callers fetch that separately via Audio).
func Create(sqlDB *sql.DB, id, title, position, mimeType string, durationMs int64, blob []byte) (*models.Note, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	seq, err := db.NextServerSeq(tx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()

	n := &models.Note{
		ID:              id,
		Kind:            "audio",
		Title:           title,
		Color:           "default",
		Position:        position,
		ArchivePosition: "",
		AudioMimeType:   mimeType,
		AudioDurationMs: durationMs,
		ServerSeq:       seq,
		CreatedAt:       now,
		UpdatedAt:       now,
		Items:           []models.ChecklistItem{},
		LabelIDs:        []string{},
	}

	if _, err = tx.Exec(`INSERT INTO notes (id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, field_clocks, server_seq, created_at, updated_at, audio_blob, audio_mime_type, audio_duration_ms)
		VALUES (?, ?, ?, '', ?, 0, 0, NULL, ?, ?, '{}', ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Kind, n.Title, n.Color, n.Position, n.ArchivePosition, n.ServerSeq, n.CreatedAt, n.UpdatedAt, blob, n.AudioMimeType, n.AudioDurationMs); err != nil {
		return nil, err
	}

	if err := searchsvc.ReindexNote(tx, n.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return n, nil
}

// Audio returns the raw recording bytes and MIME type for id. Returns
// sql.ErrNoRows if the note doesn't exist or isn't an Audio Note.
func Audio(sqlDB *sql.DB, id string) ([]byte, string, error) {
	var blob []byte
	var mimeType string
	err := sqlDB.QueryRow(`SELECT audio_blob, audio_mime_type FROM notes WHERE id = ? AND kind = 'audio'`, id).Scan(&blob, &mimeType)
	if err != nil {
		return nil, "", err
	}
	return blob, mimeType, nil
}
