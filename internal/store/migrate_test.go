package store

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestMigrationsFreshAndRepeatedOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyph.db")
	for i := 0; i < 2; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		var version int64
		if err := s.db.QueryRow(`SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version != schemaVersion {
			t.Fatalf("schema version = %d, want %d", version, schemaVersion)
		}
		s.Close()
	}
}

func TestMigrateLegacyStorePreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyph.db")
	db := openRawDB(t, path)
	applyThrough(t, db, 1)
	if _, err := db.Exec(`INSERT INTO glyphs (id, body, type, created_at, updated_at) VALUES ('g-old1','legacy searchable body','note',1,1)`); err != nil {
		t.Fatal(err)
	}
	// Releases before versioned migrations have the v1 schema but no ledger.
	if _, err := db.Exec(`DROP TABLE goose_db_version`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g, err := s.GetGlyph("g-old1")
	if err != nil || g.Body != "legacy searchable body" || g.Summary != "" || g.Starred {
		t.Fatalf("legacy glyph = %+v err=%v", g, err)
	}
	hits, err := s.Search("legacy", 5, nil, ListFilter{})
	if err != nil || len(hits) != 1 || hits[0].ID != "g-old1" {
		t.Fatalf("rebuilt FTS hits=%v err=%v", hits, err)
	}
}

func TestMigrateRejectsNewerStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyph.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO goose_db_version (version_id, is_applied, tstamp) VALUES (999, 1, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	_, err = Open(path)
	if err == nil || !strings.Contains(err.Error(), "newer than this glyph binary") {
		t.Fatalf("newer store err = %v", err)
	}
}

func TestConcurrentOpenMigratesOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyph.db")
	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			s, err := Open(path)
			if err == nil {
				err = s.Close()
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	db := openRawDB(t, filepath.Join(t.TempDir(), "glyph.db"))
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectSQLite3, db, fstest.MapFS{
		"00001_fail.sql": {Data: []byte("-- +goose Up\nCREATE TABLE partial (id INTEGER);\nTHIS IS NOT SQL;\n")},
	}, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(context.Background()); err == nil {
		t.Fatal("broken migration succeeded")
	}
	var tables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'partial'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatal("partial migration survived rollback")
	}
}

func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func applyThrough(t *testing.T, db *sql.DB, version int64) {
	t.Helper()
	f, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, db, f, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(context.Background(), version); err != nil {
		t.Fatal(err)
	}
}
