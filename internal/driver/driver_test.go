package driver

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

func TestCreateDriver(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"name":"Bob","phone":"+5678","vehicle_plate":"XYZ-001"}`))
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
	if body["vehicle_plate"] != "XYZ-001" {
		t.Fatalf("vehicle_plate: got %v", body["vehicle_plate"])
	}
}

func TestGetDriver(t *testing.T) {
	r := newTestRouter(t)

	// Create
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"name":"Bob","phone":"+5678","vehicle_plate":"XYZ-001"}`))
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
	if getBody["vehicle_plate"] != "XYZ-001" {
		t.Fatalf("vehicle_plate: got %v", getBody["vehicle_plate"])
	}
}

func TestGetDriverNotFound(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
}
