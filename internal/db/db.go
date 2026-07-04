// Package db provides access to the RideNow SQLite datastore using the
// pure-Go modernc.org/sqlite driver (no CGO required).
package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at the given DSN and verifies connectivity.
// Use ":memory:" for an ephemeral, in-memory database (handy in tests).
func Open(dsn string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dsn, err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", dsn, err)
	}
	return database, nil
}

// schema holds the idempotent DDL applied at startup. Using
// "CREATE TABLE IF NOT EXISTS" makes Migrate safe to re-run on an
// already-migrated database, which is our migration strategy (no
// external migration tooling or dependencies).
const schema = `
CREATE TABLE IF NOT EXISTS rides (
	id          TEXT PRIMARY KEY,
	rider       TEXT NOT NULL,
	origin      TEXT NOT NULL,
	destination TEXT NOT NULL,
	status      TEXT NOT NULL,
	created_at  TEXT NOT NULL
);`

// Migrate applies the schema to the database inside a transaction. It is
// idempotent: running it against an already-migrated database is a no-op.
func Migrate(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate rides: %w", err)
	}
	if _, err := tx.ExecContext(ctx, schema); err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate rides: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate rides: %w", err)
	}
	return nil
}

// newID returns a random 128-bit identifier as a hex string, using
// crypto/rand so we avoid pulling in a UUID dependency.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
