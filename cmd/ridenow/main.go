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
	dsn := getenv("RIDENOW_DB_DSN", "ridenow.db")
	addr := getenv("RIDENOW_ADDR", ":8080")

	database, err := db.Open(dsn)
	if err != nil {
		return err
	}
	defer database.Close()

	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(database),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Shut down gracefully on SIGINT/SIGTERM.
	idleClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown: %v", err)
		}
		close(idleClosed)
	}()

	log.Printf("RideNow listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-idleClosed
	log.Println("RideNow stopped")
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
