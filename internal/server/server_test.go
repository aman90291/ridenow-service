package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
