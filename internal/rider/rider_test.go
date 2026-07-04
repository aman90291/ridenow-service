package rider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ridenow/ridenow/internal/db"
)

func newTestRouter(t *testing.T) chi.Router {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := chi.NewRouter()
	New(database).Routes(r)
	return r
}

func TestCreateRider(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"name":"Alice","phone":"+1234"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want 201; body: %s", rec.Code, rec.Body)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id, _ := body["id"].(string); id == "" {
		t.Fatal("id should be non-empty")
	}
	if body["name"] != "Alice" {
		t.Fatalf("name: got %v", body["name"])
	}
}

func TestGetRider(t *testing.T) {
	r := newTestRouter(t)

	// Create
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"name":"Alice","phone":"+1234"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	id := body["id"].(string)

	// Get
	getReq := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: got %d; body: %s", getRec.Code, getRec.Body)
	}
	var getBody map[string]any
	if err := json.NewDecoder(getRec.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if getBody["id"] != id {
		t.Fatalf("id: got %v, want %s", getBody["id"], id)
	}
	if getBody["name"] != "Alice" {
		t.Fatalf("name: got %v", getBody["name"])
	}
}

func TestGetRiderNotFound(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
}
