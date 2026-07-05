// Command ridenow starts the RideNow HTTP service.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ridenow/ridenow/internal/db"
	"github.com/ridenow/ridenow/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dsn := getenv("RIDENOW_DB_DSN", "file:ridenow.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	addr := getenv("RIDENOW_ADDR", ":8080")

	database, err := db.Open(dsn)
	if err != nil {
		return err
	}
	defer database.Close()

	// Apply the schema before serving so the service never runs against an
	// unmigrated database. Migrate is idempotent and safe to re-run.
	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelMigrate()
	if err := db.Migrate(migrateCtx, database); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(database),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Shut down gracefully on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	log.Printf("RideNow listening on %s", addr)

	select {
	case err := <-srvErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
	}

	log.Println("RideNow stopped")
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
