package server

import (
	"net/http"
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
