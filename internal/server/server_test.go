package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ridenow/ridenow/internal/db"
)

func TestHealthzOK(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	srv := New(database)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := body.Status, "ok"; got != want {
		t.Fatalf("status field: got %q, want %q", got, want)
	}
	if got, want := body.DB, "ok"; got != want {
		t.Fatalf("db field: got %q, want %q", got, want)
	}
}

func TestRidesEndToEnd(t *testing.T) {
	// A file DSN avoids the modernc :memory: trap where each pooled
	// connection would otherwise see its own separate database.
	dsn := "file:" + filepath.Join(t.TempDir(), "ridenow-test.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	srv := New(database)

	// Create a ride.
	payload := `{"rider":"alice","origin":"downtown","destination":"airport"}`
	req := httptest.NewRequest(http.MethodPost, "/rides", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d", rec.Code, http.StatusCreated)
	}

	var created db.Ride
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create: expected server-generated id, got empty")
	}
	if created.Rider != "alice" || created.Origin != "downtown" || created.Destination != "airport" {
		t.Fatalf("create: unexpected persisted fields: %+v", created)
	}
	if created.Status != "requested" {
		t.Fatalf("create: status = %q, want %q", created.Status, "requested")
	}

	// Read it back by id across the pooled connections.
	getReq := httptest.NewRequest(http.MethodGet, "/rides/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status: got %d, want %d", getRec.Code, http.StatusOK)
	}

	var got db.Ride
	if err := json.NewDecoder(getRec.Body).Decode(&got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got != created {
		t.Fatalf("get: got %+v, want %+v", got, created)
	}
}

func TestGetRideNotFound(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "ridenow-test.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	srv := New(database)

	req := httptest.NewRequest(http.MethodGet, "/rides/does-not-exist", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("get unknown status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}
