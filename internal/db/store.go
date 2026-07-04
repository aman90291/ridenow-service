package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned by Store.Get when no ride matches the given id.
// Callers can map it to an HTTP 404.
var ErrNotFound = errors.New("ride not found")

// Ride is a single ride request persisted in the rides table.
type Ride struct {
	ID          string `json:"id"`
	Rider       string `json:"rider"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// Store provides persistence for rides over a shared *sql.DB pool.
type Store struct {
	db *sql.DB
}

// NewStore constructs a Store backed by the given database pool.
func NewStore(database *sql.DB) *Store {
	return &Store{db: database}
}

// Create inserts a ride and returns the stored record. It generates an id
// when none is supplied, defaults Status to "requested", and stamps
// CreatedAt with the current UTC time in RFC3339 format.
func (s *Store) Create(ctx context.Context, r Ride) (Ride, error) {
	if r.ID == "" {
		id, err := newID()
		if err != nil {
			return Ride{}, err
		}
		r.ID = id
	}
	if r.Status == "" {
		r.Status = "requested"
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	const query = `
INSERT INTO rides (id, rider, origin, destination, status, created_at)
VALUES (?, ?, ?, ?, ?, ?)`
	if _, err := s.db.ExecContext(ctx, query, r.ID, r.Rider, r.Origin, r.Destination, r.Status, r.CreatedAt); err != nil {
		return Ride{}, fmt.Errorf("create ride: %w", err)
	}
	return r, nil
}

// Get returns the ride with the given id, or ErrNotFound if it does not exist.
func (s *Store) Get(ctx context.Context, id string) (Ride, error) {
	const query = `
SELECT id, rider, origin, destination, status, created_at
FROM rides
WHERE id = ?`
	var r Ride
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID, &r.Rider, &r.Origin, &r.Destination, &r.Status, &r.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Ride{}, ErrNotFound
	}
	if err != nil {
		return Ride{}, fmt.Errorf("get ride: %w", err)
	}
	return r, nil
}
