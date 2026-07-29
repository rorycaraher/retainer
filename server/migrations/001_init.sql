CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL
);

CREATE TABLE sync_counter (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  value INTEGER NOT NULL
);
INSERT INTO sync_counter (id, value) VALUES (1, 0);

CREATE TABLE notes (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN ('text', 'checklist')),
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
  updated_at INTEGER NOT NULL
);
CREATE INDEX idx_notes_seq ON notes(server_seq);
CREATE INDEX idx_notes_trashed_at ON notes(trashed_at);

CREATE TABLE checklist_items (
  id TEXT PRIMARY KEY,
  note_id TEXT NOT NULL REFERENCES notes(id),
  text TEXT NOT NULL DEFAULT '',
  checked INTEGER NOT NULL DEFAULT 0,
  deleted INTEGER NOT NULL DEFAULT 0,
  position TEXT NOT NULL,
  field_clocks TEXT NOT NULL DEFAULT '{}',
  server_seq INTEGER NOT NULL
);
CREATE INDEX idx_items_note ON checklist_items(note_id);
CREATE INDEX idx_items_seq ON checklist_items(server_seq);

CREATE TABLE labels (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  created_at INTEGER NOT NULL
);

CREATE TABLE note_labels (
  note_id TEXT NOT NULL REFERENCES notes(id),
  label_id TEXT NOT NULL REFERENCES labels(id),
  attached INTEGER NOT NULL DEFAULT 1,
  hlc TEXT NOT NULL,
  server_seq INTEGER NOT NULL,
  PRIMARY KEY (note_id, label_id)
);
CREATE INDEX idx_note_labels_seq ON note_labels(server_seq);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  last_used_at INTEGER NOT NULL
);

CREATE TABLE config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE VIRTUAL TABLE notes_fts USING fts5(
  note_id UNINDEXED,
  title,
  content_blob
);
