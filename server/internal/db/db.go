// Package db opens the SQLite database and applies embedded migrations.
package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"retainer/server/migrations"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database at path and applies any pending migrations.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite has a single writer; a single connection avoids SQLITE_BUSY
	// under concurrent access from Go's connection pool.
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// NextServerSeq atomically increments and returns the single global
// server_seq counter, used by every service that writes notes/items/labels
// so "what changed since I last synced" stays comparable across entity types.
func NextServerSeq(tx *sql.Tx) (int64, error) {
	if _, err := tx.Exec(`UPDATE sync_counter SET value = value + 1 WHERE id = 1`); err != nil {
		return 0, err
	}
	var seq int64
	if err := tx.QueryRow(`SELECT value FROM sync_counter WHERE id = 1`).Scan(&seq); err != nil {
		return 0, err
	}
	return seq, nil
}

// CurrentServerSeq returns the counter's current value without incrementing it.
func CurrentServerSeq(tx *sql.Tx) (int64, error) {
	var seq int64
	if err := tx.QueryRow(`SELECT value FROM sync_counter WHERE id = 1`).Scan(&seq); err != nil {
		return 0, err
	}
	return seq, nil
}

func migrate(db *sql.DB) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}

	type migration struct {
		version int
		name    string
	}
	var migList []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		versionStr := strings.SplitN(e.Name(), "_", 2)[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return fmt.Errorf("migration %s has non-numeric version prefix: %w", e.Name(), err)
		}
		migList = append(migList, migration{version: version, name: e.Name()})
	}
	sort.Slice(migList, func(i, j int) bool { return migList[i].version < migList[j].version })

	// schema_migrations itself is created by 001_init.sql, so the very
	// first run has no tracking table yet — check for it explicitly.
	hasTable := false
	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'`)
	var name string
	if err := row.Scan(&name); err == nil {
		hasTable = true
	}

	applied := map[int]bool{}
	if hasTable {
		rows, err := db.Query(`SELECT version FROM schema_migrations`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var v int
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return err
			}
			applied[v] = true
		}
		rows.Close()
	}

	for _, m := range migList {
		if applied[m.version] {
			continue
		}
		sqlBytes, err := migrations.FS.ReadFile(m.name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
		// 001_init.sql creates schema_migrations itself but doesn't
		// insert its own row.
		if _, err := tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UnixMilli()); err != nil {
			tx.Rollback()
			return fmt.Errorf("record %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", m.name, err)
		}
	}
	return nil
}
