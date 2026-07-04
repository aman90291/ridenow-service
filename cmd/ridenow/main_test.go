package main

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestGetenv(t *testing.T) {
	const key = "RIDENOW_TEST_GETENV"

	t.Run("returns fallback when the variable is unset", func(t *testing.T) {
		os.Unsetenv(key)
		if got := getenv(key, "fallback"); got != "fallback" {
			t.Fatalf("getenv(%q, %q) = %q, want %q", key, "fallback", got, "fallback")
		}
	})

	t.Run("returns fallback when the variable is empty", func(t *testing.T) {
		// An explicitly empty value is treated the same as unset.
		t.Setenv(key, "")
		if got := getenv(key, "fallback"); got != "fallback" {
			t.Fatalf("getenv(%q, %q) = %q, want %q", key, "fallback", got, "fallback")
		}
	})

	t.Run("returns the value when the variable is set", func(t *testing.T) {
		t.Setenv(key, "override")
		if got := getenv(key, "fallback"); got != "override" {
			t.Fatalf("getenv(%q, %q) = %q, want %q", key, "fallback", got, "override")
		}
	})
}

// testDSN returns a DSN backed by a throwaway on-disk SQLite database in the
// same shape run() expects, so db.Open + db.Migrate succeed during the test.
func testDSN(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ridenow.db")
	return "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

// waitForListen blocks until addr accepts a TCP connection or timeout elapses.
func waitForListen(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server never started listening on %s within %s", addr, timeout)
}

// TestRunGracefulShutdown is the happy path: run() opens the database, applies
// the schema, serves HTTP, and then returns nil once it receives SIGTERM.
func TestRunGracefulShutdown(t *testing.T) {
	const addr = "127.0.0.1:38571"
	t.Setenv("RIDENOW_DB_DSN", testDSN(t))
	t.Setenv("RIDENOW_ADDR", addr)

	errCh := make(chan error, 1)
	go func() { errCh <- run() }()

	// run() installs the signal handler before ListenAndServe starts, so once the
	// port accepts connections we know SIGTERM will be caught rather than killing
	// the test binary via the default disposition.
	waitForListen(t, addr, 5*time.Second)

	// Guard against run() having already exited (e.g. the port was taken); in that
	// case sending SIGTERM to ourselves would be unhandled.
	select {
	case err := <-errCh:
		t.Fatalf("run() exited before shutdown was requested: %v", err)
	default:
	}

	proc, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("sending SIGTERM: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run() returned error on graceful shutdown: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run() did not return within 15s of SIGTERM")
	}
}

// TestRunReturnsListenError is the error path: a valid database but an
// unbindable listen address must surface as a non-nil error from run().
func TestRunReturnsListenError(t *testing.T) {
	t.Setenv("RIDENOW_DB_DSN", testDSN(t))
	t.Setenv("RIDENOW_ADDR", "127.0.0.1:99999") // port out of range

	done := make(chan error, 1)
	go func() { done <- run() }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("run() = nil, want error for an invalid listen address")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run() blocked instead of returning the listen error")
	}
}
