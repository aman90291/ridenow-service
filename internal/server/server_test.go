package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

// TestTripsEndToEnd exercises the modular-monolith wiring end to end:
// auth (transport) -> trip (domain) -> rider/driver validation -> persistence.
// It proves the protected POST /trips endpoint enforces the bearer token at
// the transport boundary before any trip domain code runs, persists a valid
// request, and rejects an invalid driver id.
func TestTripsEndToEnd(t *testing.T) {
	srv := newTestServer(t)

	t.Run("valid token creates a trip and it reads back", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(`{"driver_id":"driver-1"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer rider-1")
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
		// The authenticated rider comes from the token; the driver from the body.
		if created.Rider != "rider-1" || created.Origin != "driver-1" {
			t.Fatalf("create: unexpected persisted fields: %+v", created)
		}

		// Read the persisted trip back by id across the pooled connections.
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
	})

	t.Run("missing token is rejected before reaching the domain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(`{"driver_id":"driver-1"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
		}

		// The 401 above is what proves nothing was persisted: the request is
		// rejected at the auth middleware before the trip handler runs, so no
		// id is ever generated. This GET is only a sanity check that an id we
		// never created 404s; it can't by itself establish the negative.
		getReq := httptest.NewRequest(http.MethodGet, "/rides/should-not-exist", nil)
		getRec := httptest.NewRecorder()
		srv.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusNotFound {
			t.Fatalf("unexpected persistence after unauthenticated request: got %d", getRec.Code)
		}
	})

	t.Run("valid token but empty driver_id is a 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/trips", bytes.NewBufferString(`{"driver_id":""}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer rider-1")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
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
