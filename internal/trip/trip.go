package trip

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ridenow/ridenow/internal/domain"
)

type Handler struct {
	db *sql.DB
}

func New(db *sql.DB) *Handler {
	h := &Handler{db: db}
	if err := h.schema(); err != nil {
		panic("trip: schema migration: " + err.Error())
	}
	return h
}

func (h *Handler) schema() error {
	_, err := h.db.Exec(`CREATE TABLE IF NOT EXISTS trips (
		id          TEXT PRIMARY KEY,
		rider_id    TEXT NOT NULL,
		driver_id   TEXT NOT NULL,
		status      TEXT NOT NULL,
		origin      TEXT NOT NULL,
		destination TEXT NOT NULL
	)`)
	return err
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.handleCreate)
	r.Get("/{id}", h.handleGet)
}

type tripResponse struct {
	ID          string          `json:"id"`
	RiderID     domain.RiderID  `json:"rider_id"`
	DriverID    domain.DriverID `json:"driver_id"`
	Status      string          `json:"status"`
	Origin      string          `json:"origin"`
	Destination string          `json:"destination"`
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RiderID     domain.RiderID  `json:"rider_id"`
		DriverID    domain.DriverID `json:"driver_id"`
		Origin      string          `json:"origin"`
		Destination string          `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.RiderID == "" || req.DriverID == "" {
		http.Error(w, "rider_id and driver_id are required", http.StatusBadRequest)
		return
	}

	id := uuid.NewString()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	_, err := h.db.ExecContext(ctx,
		`INSERT INTO trips (id, rider_id, driver_id, status, origin, destination)
		 VALUES (?, ?, ?, 'requested', ?, ?)`,
		id, req.RiderID, req.DriverID, req.Origin, req.Destination,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, tripResponse{
		ID:          id,
		RiderID:     req.RiderID,
		DriverID:    req.DriverID,
		Status:      "requested",
		Origin:      req.Origin,
		Destination: req.Destination,
	})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var resp tripResponse
	err := h.db.QueryRowContext(ctx,
		`SELECT id, rider_id, driver_id, status, origin, destination FROM trips WHERE id = ?`, id,
	).Scan(&resp.ID, &resp.RiderID, &resp.DriverID, &resp.Status, &resp.Origin, &resp.Destination)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("trip: encode response: %v", err)
	}
}
