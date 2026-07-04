// Package server wires up the RideNow HTTP router, middleware, and handlers.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ridenow/ridenow/internal/db"
)

// Server holds application dependencies and the HTTP router.
type Server struct {
	db     *sql.DB
	store  *db.Store
	router chi.Router
}

// New constructs a Server with middleware and routes wired up.
func New(database *sql.DB) *Server {
	s := &Server{
		db:     database,
		store:  db.NewStore(database),
		router: chi.NewRouter(),
	}
	s.routes()
	return s
}

// ServeHTTP lets Server satisfy http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	s.router.Get("/healthz", s.handleHealth)
	s.router.Post("/rides", s.handleCreateRide)
	s.router.Get("/rides/{id}", s.handleGetRide)
}

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// handleHealth reports liveness and readiness, including a database ping.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{Status: "ok", DB: "ok"}
	code := http.StatusOK

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		resp.Status = "degraded"
		resp.DB = "unreachable"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("health: encode response: %v", err)
	}
}

// handleCreateRide persists a ride from the request body and returns it.
func (s *Server) handleCreateRide(w http.ResponseWriter, r *http.Request) {
	var ride db.Ride
	if err := json.NewDecoder(r.Body).Decode(&ride); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	created, err := s.store.Create(ctx, ride)
	if err != nil {
		log.Printf("rides: create: %v", err)
		http.Error(w, "could not create ride", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(created); err != nil {
		log.Printf("rides: encode response: %v", err)
	}
}

// handleGetRide returns a persisted ride by id, or 404 if it is unknown.
func (s *Server) handleGetRide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ride, err := s.store.Get(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		http.Error(w, "ride not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("rides: get: %v", err)
		http.Error(w, "could not fetch ride", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ride); err != nil {
		log.Printf("rides: encode response: %v", err)
	}
}
