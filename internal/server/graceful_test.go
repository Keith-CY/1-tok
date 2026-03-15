package server

import (
	"testing"
	"time"
	"net/http"
)

func TestDefaultShutdownTimeout(t *testing.T) {
	if DefaultShutdownTimeout != 30*time.Second {
		t.Errorf("default = %v", DefaultShutdownTimeout)
	}
}

func TestListenAndServeGraceful_InvalidAddr(t *testing.T) {
	// Use an invalid address to make ListenAndServe fail immediately
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	err := ListenAndServeGraceful("invalid-not-a-port", handler, time.Second)
	if err == nil {
		t.Error("expected error for invalid address")
	}
}
