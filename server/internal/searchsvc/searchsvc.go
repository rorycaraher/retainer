// Package searchsvc maintains the denormalized, app-managed notes_fts index
// (FTS5) and answers search queries against it. Reindexing happens via an
// explicit Go call inside the same transaction as any note/item write — not
// SQL triggers — so the logic stays in Go and testable (see the plan's "FTS
// maintenance" section).
package searchsvc

import (
	"database/sql"
	"strings"
)

// ReindexNote recomputes and upserts noteID's FTS row from the note and its
// checklist items' CURRENT state within tx, so a concurrent query never sees
// half-applied search content mid-transaction. Call it after any write that
// could affect a note's searchable text (title, body, or item text/deleted).
func ReindexNote(tx *sql.Tx, noteID string) error {
	var title, body, kind string
	err := tx.QueryRow(`SELECT title, body, kind FROM notes WHERE id = ?`, noteID).Scan(&title, &body, &kind)
	if err == sql.ErrNoRows {
		_, err := tx.Exec(`DELETE FROM notes_fts WHERE note_id = ?`, noteID)
		return err
	}
	if err != nil {
		return err
	}

	contentBlob := body
	if kind == "checklist" {
		rows, err := tx.Query(`SELECT text FROM checklist_items WHERE note_id = ? AND deleted = 0 ORDER BY position ASC`, noteID)
		if err != nil {
			return err
		}
		var lines []string
		for rows.Next() {
			var text string
			if err := rows.Scan(&text); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, text)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		contentBlob = strings.Join(lines, "\n")
	}

	if _, err := tx.Exec(`DELETE FROM notes_fts WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO notes_fts (note_id, title, content_blob) VALUES (?, ?, ?)`, noteID, title, contentBlob)
	return err
}

// sanitizeQuery turns free-text user input into a safe FTS5 MATCH expression:
// each whitespace-separated token is quoted (embedded quotes doubled per
// FTS5's escaping rule), so punctuation and FTS operators in the input
// (AND, OR, *, -, unbalanced quotes, ...) are treated as literal text rather
// than query syntax, and implicitly ANDed together as separate phrases.
func sanitizeQuery(q string) string {
	fields := strings.Fields(q)
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		escaped := strings.ReplaceAll(f, `"`, `""`)
		parts = append(parts, `"`+escaped+`"`)
	}
	return strings.Join(parts, " ")
}

// Search returns the ids of non-trashed, non-archived notes matching query,
// best match first — "search is a bar over the main grid" (the plan), so
// results are scoped the same way the main grid is.
func Search(db *sql.DB, query string) ([]string, error) {
	q := sanitizeQuery(query)
	if q == "" {
		return []string{}, nil
	}

	rows, err := db.Query(`SELECT f.note_id FROM notes_fts f
		JOIN notes n ON n.id = f.note_id
		WHERE notes_fts MATCH ? AND n.trashed_at IS NULL AND n.archived = 0
		ORDER BY rank`, q)
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
