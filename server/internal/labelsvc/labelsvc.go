// Package labelsvc implements plain REST CRUD for Labels (CONTEXT.md: a
// first-class, user-managed entity). There's no meaningful merge scenario
// for label management itself, unlike note<->label attachment (which goes
// through syncsvc as a regular field-level mutation) — renaming/deleting a
// label is a rare, deliberate, single-user action.
package labelsvc

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"retainer/server/internal/models"
)

var ErrNotFound = errors.New("label not found")
var ErrNameTaken = errors.New("a label with that name already exists")

// List returns every Label ordered by name.
func List(db *sql.DB) ([]*models.Label, error) {
	rows, err := db.Query(`SELECT id, name, created_at FROM labels ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	labels := []*models.Label{}
	for rows.Next() {
		l := &models.Label{}
		if err := rows.Scan(&l.ID, &l.Name, &l.CreatedAt); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// Create adds a new Label. Name must be non-empty and unique.
func Create(db *sql.DB, name string, now int64) (*models.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("label name must not be empty")
	}
	l := &models.Label{ID: uuid.NewString(), Name: name, CreatedAt: now}
	_, err := db.Exec(`INSERT INTO labels (id, name, created_at) VALUES (?, ?, ?)`, l.ID, l.Name, l.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrNameTaken
		}
		return nil, err
	}
	return l, nil
}

// Rename updates a Label's name in place. Since notes reference labels by
// id (a real foreign key, not a denormalized name copy), this alone makes
// the new name show up everywhere the label is attached — no propagation
// step needed beyond the single UPDATE.
func Rename(db *sql.DB, id, name string) (*models.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("label name must not be empty")
	}
	res, err := db.Exec(`UPDATE labels SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrNameTaken
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	var createdAt int64
	if err := db.QueryRow(`SELECT created_at FROM labels WHERE id = ?`, id).Scan(&createdAt); err != nil {
		return nil, err
	}
	return &models.Label{ID: id, Name: name, CreatedAt: createdAt}, nil
}

// Delete removes a Label and strips it from every Note it was attached to.
func Delete(db *sql.DB, id string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM note_labels WHERE label_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM labels WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
