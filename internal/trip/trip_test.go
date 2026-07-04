package trip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ridenow/ridenow/internal/auth"
	"github.com/ridenow/ridenow/internal/db"
	"github.com/ridenow/ridenow/internal/driver"
	"github.com/ridenow/ridenow/internal/rider"
)

func newTestRouter(t *testing.T) (chi.Router, *Handler) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	authH := auth.New(database)
	riderH := rider.New(database)
	driverH := driver.New(database)
	tripH := New(database)

	r := chi.NewRouter()
	r.Route("/auth", authH.Routes)
	r.Route("/riders", riderH.Routes)
	r.Route("/drivers", driverH.Routes)
	r.Route("/trips", tripH.Routes)

	return r, tripH
}

func TestEndToEnd(t *testing.T) {
	r, _ := newTestRouter(t)

	// Create rider
	rReq := httptest.NewRequest(http.MethodPost, "/riders",
		strings.NewReader(`{"name":"Alice","phone":"+1234"}`))
	rReq.Header.Set("Content-Type", "application/json")
	rRec := httptest.NewRecorder()
	r.ServeHTTP(rRec, rReq)
	if rRec.Code != http.StatusCreated {
		t.Fatalf("create rider: got %d; body: %s", rRec.Code, rRec.Body)
	}
	var riderBody map[string]any
	if err := json.NewDecoder(rRec.Body).Decode(&riderBody); err != nil {
		t.Fatalf("decode rider: %v", err)
	}
	riderID := riderBody["id"].(string)

	// Create driver
	dReq := httptest.NewRequest(http.MethodPost, "/drivers",
		strings.NewReader(`{"name":"Bob","phone":"+5678","vehicle_plate":"XYZ-001"}`))
	dReq.Header.Set("Content-Type", "application/json")
	dRec := httptest.NewRecorder()
	r.ServeHTTP(dRec, dReq)
	if dRec.Code != http.StatusCreated {
		t.Fatalf("create driver: got %d; body: %s", dRec.Code, dRec.Body)
	}
	var driverBody map[string]any
	if err := json.NewDecoder(dRec.Body).Decode(&driverBody); err != nil {
		t.Fatalf("decode driver: %v", err)
	}
	driverID := driverBody["id"].(string)

	// Create trip
	tripPayload := fmt.Sprintf(
		`{"rider_id":%q,"driver_id":%q,"origin":"Downtown","destination":"Airport"}`,
		riderID, driverID,
	)
	tReq := httptest.NewRequest(http.MethodPost, "/trips", strings.NewReader(tripPayload))
	tReq.Header.Set("Content-Type", "application/json")
	tRec := httptest.NewRecorder()
	r.ServeHTTP(tRec, tReq)
	if tRec.Code != http.StatusCreated {
		t.Fatalf("create trip: got %d; body: %s", tRec.Code, tRec.Body)
	}
	var tripBody map[string]any
	if err := json.NewDecoder(tRec.Body).Decode(&tripBody); err != nil {
		t.Fatalf("decode trip: %v", err)
	}
	if tripBody["rider_id"] != riderID {
		t.Fatalf("rider_id: got %v, want %s", tripBody["rider_id"], riderID)
	}
	if tripBody["driver_id"] != driverID {
		t.Fatalf("driver_id: got %v, want %s", tripBody["driver_id"], driverID)
	}
	if tripBody["status"] != "requested" {
		t.Fatalf("status: got %v, want requested", tripBody["status"])
	}
	tripID := tripBody["id"].(string)

	// GET /trips/{id}
	gReq := httptest.NewRequest(http.MethodGet, "/trips/"+tripID, nil)
	gRec := httptest.NewRecorder()
	r.ServeHTTP(gRec, gReq)
	if gRec.Code != http.StatusOK {
		t.Fatalf("get trip: got %d; body: %s", gRec.Code, gRec.Body)
	}
	var getBody map[string]any
	if err := json.NewDecoder(gRec.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get trip: %v", err)
	}
	if getBody["rider_id"] != riderID {
		t.Fatalf("get rider_id: got %v, want %s", getBody["rider_id"], riderID)
	}
	if getBody["driver_id"] != driverID {
		t.Fatalf("get driver_id: got %v, want %s", getBody["driver_id"], driverID)
	}
	if getBody["origin"] != "Downtown" {
		t.Fatalf("origin: got %v", getBody["origin"])
	}
	if getBody["destination"] != "Airport" {
		t.Fatalf("destination: got %v", getBody["destination"])
	}
}

func TestCreateTripMissingRiderID(t *testing.T) {
	r, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/trips",
		strings.NewReader(`{"driver_id":"some-driver","origin":"A","destination":"B"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", rec.Code)
	}
}

func TestGetTripNotFound(t *testing.T) {
	r, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/trips/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
}
