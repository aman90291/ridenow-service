package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ridenow/ridenow/internal/db"
)

// newTestServer wires a Server over a fresh, migrated file-backed SQLite
// database in a temp dir. A file DSN avoids the modernc :memory: trap where
// each pooled connection would otherwise see its own separate database.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "ridenow-test.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(database)
}

// assertHealthBody decodes a /healthz response body and asserts it is exactly
// {"status":<wantStatus>,"db":<wantDB>} with no extra fields. Decoding into a
// map (rather than the healthResponse struct, which would silently ignore
// unexpected keys) locks the public health-check schema against silent drift.
func assertHealthBody(t *testing.T, body []byte, wantStatus, wantDB string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode health body: %v (body=%q)", err, body)
	}
	want := map[string]string{"status": wantStatus, "db": wantDB}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("health body: got %v, want %v", got, want)
	}
}

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
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: got %q, want %q", ct, "application/json")
	}
	assertHealthBody(t, rec.Body.Bytes(), "ok", "ok")
}

// TestHealthzDegraded pins the readiness-failure contract: when the database
// ping fails, /healthz returns 503 with {"status":"degraded","db":"unreachable"}.
// Closing the *sql.DB before the request makes PingContext fail, driving the
// degraded path without routing through the store.
func TestHealthzDegraded(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}

	srv := New(database)

	if err := database.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: got %q, want %q", ct, "application/json")
	}
	assertHealthBody(t, rec.Body.Bytes(), "degraded", "unreachable")
}

func TestRidesEndToEnd(t *testing.T) {
	srv := newTestServer(t)

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
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/rides/does-not-exist", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("get unknown status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreateRideRejectsInvalidBody proves that missing/blank required fields,
// malformed JSON, and attempts to set server-owned fields (id/status/
// created_at) are rejected with 400 rather than persisted.
func TestCreateRideRejectsInvalidBody(t *testing.T) {
	srv := newTestServer(t)

	cases := []struct {
		name    string
		payload string
	}{
		{"empty object", `{}`},
		{"blank rider", `{"rider":"","origin":"downtown","destination":"airport"}`},
		{"blank origin", `{"rider":"alice","origin":"","destination":"airport"}`},
		{"blank destination", `{"rider":"alice","origin":"downtown","destination":""}`},
		{"malformed json", `{`},
		{"server-owned status", `{"rider":"alice","origin":"downtown","destination":"airport","status":"completed"}`},
		{"server-owned id", `{"rider":"alice","origin":"downtown","destination":"airport","id":"forged"}`},
		{"server-owned created_at", `{"rider":"alice","origin":"downtown","destination":"airport","created_at":"1999-01-01T00:00:00Z"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/rides", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

// TestCreateRideRejectsOversizedBody proves the create endpoint bounds the
// request body to guard against a large-payload denial of service.
func TestCreateRideRejectsOversizedBody(t *testing.T) {
	srv := newTestServer(t)

	huge := strings.Repeat("a", maxCreateRideBody+1)
	payload := `{"rider":"` + huge + `","origin":"downtown","destination":"airport"}`
	req := httptest.NewRequest(http.MethodPost, "/rides", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
