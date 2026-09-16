package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/gofrs/flock"
	"github.com/pressly/goose/v3"
)

const schemaVersion int64 = 2

// migrationFiles are compiled into the CLI so a Glyph store never depends on
// migration files being installed beside the executable.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

func migrate(db *sql.DB, dbPath string) error {
	// Goose does not lock SQLite migrations across processes. Glyph is commonly
	// invoked by concurrent agents, so serialize the version-check/apply window
	// with a lock adjacent to the database. The file intentionally persists;
	// removing a lock file creates inode races between waiting processes.
	lock := flock.New(dbPath + ".migrate.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("migration lock: timed out waiting for another glyph process")
	}
	defer lock.Unlock() //nolint:errcheck -- the migration result is already durable

	fsys, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("migration files: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db,
		fsys,
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > target || current > schemaVersion {
		return fmt.Errorf("store schema version %d is newer than this glyph binary (supports %d); upgrade glyph", current, schemaVersion)
	}
	if target != schemaVersion {
		return fmt.Errorf("embedded migration target %d does not match supported schema version %d", target, schemaVersion)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
