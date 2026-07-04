package rider

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
		panic("rider: schema migration: " + err.Error())
	}
	return h
}

func (h *Handler) schema() error {
	_, err := h.db.Exec(`CREATE TABLE IF NOT EXISTS riders (
		id    TEXT PRIMARY KEY,
		name  TEXT NOT NULL,
		phone TEXT NOT NULL
	)`)
	return err
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.handleCreate)
	r.Get("/{id}", h.handleGet)
}

type riderResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	id := uuid.NewString()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	_, err := h.db.ExecContext(ctx,
		`INSERT INTO riders (id, name, phone) VALUES (?, ?, ?)`,
		id, req.Name, req.Phone,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, riderResponse{ID: id, Name: req.Name, Phone: req.Phone})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var resp riderResponse
	err := h.db.QueryRowContext(ctx,
		`SELECT id, name, phone FROM riders WHERE id = ?`, id,
	).Scan(&resp.ID, &resp.Name, &resp.Phone)
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
		log.Printf("rider: encode response: %v", err)
	}
}
