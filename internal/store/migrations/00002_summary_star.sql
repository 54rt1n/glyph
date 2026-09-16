-- +goose Up

ALTER TABLE glyphs ADD COLUMN summary TEXT;
ALTER TABLE glyphs ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;

CREATE INDEX glyphs_starred ON glyphs(starred) WHERE starred = 1;

DROP TRIGGER glyphs_ai;
DROP TRIGGER glyphs_ad;
DROP TRIGGER glyphs_au;
DROP TABLE glyphs_fts;

CREATE VIRTUAL TABLE glyphs_fts USING fts5(
  summary, body, content='glyphs', content_rowid='rowid'
);

-- +goose StatementBegin
CREATE TRIGGER glyphs_ai AFTER INSERT ON glyphs BEGIN
  INSERT INTO glyphs_fts(rowid, summary, body) VALUES (new.rowid, new.summary, new.body);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER glyphs_ad AFTER DELETE ON glyphs BEGIN
  INSERT INTO glyphs_fts(glyphs_fts, rowid, summary, body)
    VALUES ('delete', old.rowid, old.summary, old.body);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER glyphs_au AFTER UPDATE OF summary, body ON glyphs BEGIN
  INSERT INTO glyphs_fts(glyphs_fts, rowid, summary, body)
    VALUES ('delete', old.rowid, old.summary, old.body);
  INSERT INTO glyphs_fts(rowid, summary, body) VALUES (new.rowid, new.summary, new.body);
END;
-- +goose StatementEnd

INSERT INTO glyphs_fts(glyphs_fts) VALUES ('rebuild');
