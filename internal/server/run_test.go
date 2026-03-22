package server

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRun_ShutdownOnSIGTERM(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run("127.0.0.1:0", handler, 1*time.Second)
	}()

	time.Sleep(50 * time.Millisecond)
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error on clean shutdown, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s after SIGTERM")
	}
}

func TestRun_InvalidAddr(t *testing.T) {
	err := Run("invalid-not-a-port", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), time.Second)
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

func TestRun_GracefulShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	done := make(chan error, 1)
	go func() {
		done <- Run(":0", handler, 100*time.Millisecond)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGINT to trigger graceful shutdown
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for graceful shutdown")
	}
}

func TestRun_DefaultDrainTimeout(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		done <- Run(":0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), 0)
	}()

	time.Sleep(50 * time.Millisecond)
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGTERM)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("timed out — DefaultDrainTimeout too long")
	}
}

func TestEnvDurationSecondsOrDefault(t *testing.T) {
	t.Setenv("SERVER_WRITE_TIMEOUT_SECONDS", "180")
	if got := envDurationSecondsOrDefault("SERVER_WRITE_TIMEOUT_SECONDS", time.Minute); got != 180*time.Second {
		t.Fatalf("timeout = %s, want 3m0s", got)
	}
}

func TestEnvDurationSecondsOrDefaultFallsBackOnInvalidValue(t *testing.T) {
	t.Setenv("SERVER_WRITE_TIMEOUT_SECONDS", "not-a-number")
	if got := envDurationSecondsOrDefault("SERVER_WRITE_TIMEOUT_SECONDS", time.Minute); got != time.Minute {
		t.Fatalf("timeout = %s, want %s", got, time.Minute)
	}
}
