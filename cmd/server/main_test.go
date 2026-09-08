package main

import (
	"io"
	"log/slog"
	"syscall"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestRunShutsDownOnSignal exercises the graceful-shutdown path: run() must
// return nil after SIGTERM rather than being killed mid-flight.
func TestRunShutsDownOnSignal(t *testing.T) {
	t.Setenv("ADDR", "127.0.0.1:0")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")

	done := make(chan error, 1)
	go func() { done <- run(discardLogger()) }()

	// Give the listener a moment to come up before signalling it.
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("sending SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run() = %v, want nil after SIGTERM", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run() did not return within 10s of SIGTERM")
	}
}

// TestRunFailsOnUnusableAddr covers the error path out of ListenAndServe.
func TestRunFailsOnUnusableAddr(t *testing.T) {
	t.Setenv("ADDR", "127.0.0.1:not-a-port")

	if err := run(discardLogger()); err == nil {
		t.Error("run() = nil, want an error for an unusable address")
	}
}
