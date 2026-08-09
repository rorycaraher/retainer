-- Adds the Audio Note kind (docs/adr/0005): a single immutable recording
-- stored as a BLOB alongside its MIME type and duration. SQLite can't widen
-- an existing CHECK constraint in place, so the notes table is rebuilt.
-- (foreign_keys enforcement is turned off around this whole migration by
-- server/internal/db.migrate — see the comment there for why.)

CREATE TABLE notes_new (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('text', 'checklist', 'audio')),
  title TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL DEFAULT 'default',
  pinned INTEGER NOT NULL DEFAULT 0,
  archived INTEGER NOT NULL DEFAULT 0,
  trashed_at INTEGER,
  position TEXT NOT NULL,
  archive_position TEXT NOT NULL DEFAULT '',
  field_clocks TEXT NOT NULL DEFAULT '{}',
  server_seq INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  audio_blob BLOB,
  audio_mime_type TEXT NOT NULL DEFAULT '',
  audio_duration_ms INTEGER NOT NULL DEFAULT 0
);

INSERT INTO notes_new (id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, field_clocks, server_seq, created_at, updated_at)
SELECT id, kind, title, body, color, pinned, archived, trashed_at, position, archive_position, field_clocks, server_seq, created_at, updated_at FROM notes;

DROP TABLE notes;
ALTER TABLE notes_new RENAME TO notes;

CREATE INDEX idx_notes_seq ON notes(server_seq);
CREATE INDEX idx_notes_trashed_at ON notes(trashed_at);
