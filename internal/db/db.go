// Package db provides access to the RideNow SQLite datastore using the
// pure-Go modernc.org/sqlite driver (no CGO required).
package db

import (
	"database/sql"
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
