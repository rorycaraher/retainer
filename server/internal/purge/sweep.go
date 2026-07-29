// Package purge permanently deletes Notes that have been in Trash past
// their 7-day retention window (see CONTEXT.md's Trash definition), plus
// the explicit immediate "delete forever" action from the Trash view.
package purge

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

const retention = 7 * 24 * time.Hour

// PurgeOne immediately and permanently deletes a single note (regardless of
// its trashed_at value) and everything that references it — used by the
// explicit DELETE /api/notes/:id/purge endpoint, bypassing the 7-day wait.
func PurgeOne(db *sql.DB, id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := purgeNote(tx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Sweep deletes every note whose trashed_at is older than the retention
// window as of now, cascading to its checklist items, label attachments,
// and FTS index row. Returns how many notes were purged.
func Sweep(db *sql.DB, now time.Time) (int, error) {
	cutoff := now.Add(-retention).UnixMilli()

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id FROM notes WHERE trashed_at IS NOT NULL AND trashed_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	for _, id := range ids {
		if err := purgeNote(tx, id); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func purgeNote(tx *sql.Tx, id string) error {
	if _, err := tx.Exec(`DELETE FROM checklist_items WHERE note_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM note_labels WHERE note_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM notes_fts WHERE note_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM notes WHERE id = ?`, id); err != nil {
		return err
	}
	return nil
}

// Run ticks Sweep on interval until ctx is cancelled — start it in its own
// goroutine from main.go, alongside the WS hub, both stopped via the same
// shutdown context.
func Run(ctx context.Context, db *sql.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n, err := Sweep(db, time.Now())
			if err != nil {
				slog.Error("purge sweep", "error", err)
			} else if n > 0 {
				slog.Info("purge sweep", "removed", n)
			}
		case <-ctx.Done():
			return
		}
	}
}
