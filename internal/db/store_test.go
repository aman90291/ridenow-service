package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// testDB opens a file-backed SQLite database in a temp dir. A file DSN
// avoids the modernc :memory: trap where each pooled connection gets its
// own separate in-memory database.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "ridenow-test.db")
	database, err := Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestStoreRoundTrip(t *testing.T) {
	database := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewStore(database)
	created, err := store.Create(ctx, Ride{
		Rider:       "alice",
		Origin:      "downtown",
		Destination: "airport",
	})
	if err != nil {
		t.Fatalf("create ride: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create: expected generated id, got empty")
	}
	if created.Status != "requested" {
		t.Fatalf("create: status = %q, want %q", created.Status, "requested")
	}
	if created.CreatedAt == "" {
		t.Fatal("create: expected created_at to be set")
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get ride: %v", err)
	}
	if got != created {
		t.Fatalf("get: got %+v, want %+v", got, created)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	database := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestGetNotFound(t *testing.T) {
	database := testDB(t)
	ctx := context.Background()
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := NewStore(database)
	if _, err := store.Get(ctx, "does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get unknown: got %v, want ErrNotFound", err)
	}
}
