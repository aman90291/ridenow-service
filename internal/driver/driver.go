package driver

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
)

type Handler struct {
	db *sql.DB
}

func New(db *sql.DB) *Handler {
	h := &Handler{db: db}
	if err := h.schema(); err != nil {
		panic("driver: schema migration: " + err.Error())
	}
	return h
}

func (h *Handler) schema() error {
	_, err := h.db.Exec(`CREATE TABLE IF NOT EXISTS drivers (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		phone         TEXT NOT NULL,
		vehicle_plate TEXT NOT NULL
	)`)
	return err
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.handleCreate)
	r.Get("/{id}", h.handleGet)
}

type driverResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	VehiclePlate string `json:"vehicle_plate"`
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Phone        string `json:"phone"`
		VehiclePlate string `json:"vehicle_plate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	id := uuid.NewString()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	_, err := h.db.ExecContext(ctx,
		`INSERT INTO drivers (id, name, phone, vehicle_plate) VALUES (?, ?, ?, ?)`,
		id, req.Name, req.Phone, req.VehiclePlate,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, driverResponse{
		ID:           id,
		Name:         req.Name,
		Phone:        req.Phone,
		VehiclePlate: req.VehiclePlate,
	})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var resp driverResponse
	err := h.db.QueryRowContext(ctx,
		`SELECT id, name, phone, vehicle_plate FROM drivers WHERE id = ?`, id,
	).Scan(&resp.ID, &resp.Name, &resp.Phone, &resp.VehiclePlate)
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
		log.Printf("driver: encode response: %v", err)
	}
}
