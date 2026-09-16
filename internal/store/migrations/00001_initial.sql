-- +goose Up

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

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS glyphs_ai AFTER INSERT ON glyphs BEGIN
  INSERT INTO glyphs_fts(rowid, body) VALUES (new.rowid, new.body);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS glyphs_ad AFTER DELETE ON glyphs BEGIN
  INSERT INTO glyphs_fts(glyphs_fts, rowid, body) VALUES ('delete', old.rowid, old.body);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS glyphs_au AFTER UPDATE OF body ON glyphs BEGIN
  INSERT INTO glyphs_fts(glyphs_fts, rowid, body) VALUES ('delete', old.rowid, old.body);
  INSERT INTO glyphs_fts(rowid, body) VALUES (new.rowid, new.body);
END;
-- +goose StatementEnd
