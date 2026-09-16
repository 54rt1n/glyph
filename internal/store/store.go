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

// Open opens (creating if needed) the database at path, in WAL mode with a
// busy timeout, and applies pending embedded migrations.
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
	if err := migrate(db, path); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
