// Package server wires up the RideNow HTTP router, middleware, and handlers.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ridenow/ridenow/internal/auth"
	"github.com/ridenow/ridenow/internal/driver"
	"github.com/ridenow/ridenow/internal/rider"
	"github.com/ridenow/ridenow/internal/trip"
)

// Server holds application dependencies and the HTTP router.
type Server struct {
	db     *sql.DB
	router chi.Router
}

// New constructs a Server with middleware and routes wired up.
func New(db *sql.DB) *Server {
	s := &Server{
		db:     db,
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

	s.router.Route("/auth", auth.New(s.db).Routes)
	s.router.Route("/riders", rider.New(s.db).Routes)
	s.router.Route("/drivers", driver.New(s.db).Routes)
	s.router.Route("/trips", trip.New(s.db).Routes)
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
