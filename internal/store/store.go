// Package store owns the SQLite database: schema, CRUD, FTS, vectors.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS glyphs (
  id         TEXT PRIMARY KEY,
  body       TEXT NOT NULL,
  type       TEXT,
  meta       TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS edges (
  id         TEXT PRIMARY KEY,
  src        TEXT NOT NULL REFERENCES glyphs(id) ON DELETE CASCADE,
  dst        TEXT NOT NULL REFERENCES glyphs(id) ON DELETE CASCADE,
  rel        TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  UNIQUE (src, dst, rel)
);

CREATE TABLE IF NOT EXISTS glyph_tags (
  glyph_id   TEXT NOT NULL REFERENCES glyphs(id) ON DELETE CASCADE,
  tag        TEXT NOT NULL,
  PRIMARY KEY (glyph_id, tag)
);

CREATE TABLE IF NOT EXISTS glyph_refs (
  id         TEXT PRIMARY KEY,
  glyph_id   TEXT NOT NULL REFERENCES glyphs(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL,
  target     TEXT NOT NULL,
  label      TEXT,
  created_at INTEGER NOT NULL,
  UNIQUE (glyph_id, kind, target)
);

CREATE TABLE IF NOT EXISTS glyph_vecs (
  glyph_id   TEXT PRIMARY KEY REFERENCES glyphs(id) ON DELETE CASCADE,
  model      TEXT NOT NULL,
  dims       INTEGER NOT NULL,
  vec        BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS edges_src ON edges(src);
CREATE INDEX IF NOT EXISTS edges_dst ON edges(dst);
CREATE INDEX IF NOT EXISTS glyphs_created ON glyphs(created_at);
CREATE INDEX IF NOT EXISTS glyphs_type ON glyphs(type);
CREATE INDEX IF NOT EXISTS refs_target ON glyph_refs(kind, target);

CREATE VIRTUAL TABLE IF NOT EXISTS glyphs_fts USING fts5(
  body, content='glyphs', content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS glyphs_ai AFTER INSERT ON glyphs BEGIN
  INSERT INTO glyphs_fts(rowid, body) VALUES (new.rowid, new.body);
END;
CREATE TRIGGER IF NOT EXISTS glyphs_ad AFTER DELETE ON glyphs BEGIN
  INSERT INTO glyphs_fts(glyphs_fts, rowid, body) VALUES ('delete', old.rowid, old.body);
END;
CREATE TRIGGER IF NOT EXISTS glyphs_au AFTER UPDATE OF body ON glyphs BEGIN
  INSERT INTO glyphs_fts(glyphs_fts, rowid, body) VALUES ('delete', old.rowid, old.body);
  INSERT INTO glyphs_fts(rowid, body) VALUES (new.rowid, new.body);
END;
`

// Open opens (creating if needed) the database at path, in WAL mode with a
// busy timeout, and applies the schema.
func Open(path string) (*Store, error) {
	// _txlock=immediate: write transactions take the write lock at BEGIN, so
	// concurrent processes wait on busy_timeout instead of failing SQLITE_BUSY
	// on the deferred read→write upgrade.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection avoids SQLITE_BUSY between our own statements; the CLI
	// is one process per command, so pooling buys nothing.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
