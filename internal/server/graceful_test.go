package server

import (
	"testing"
	"time"
	"net/http"
	"net"
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

func TestListenAndServeGraceful_PortInUse(t *testing.T) {
	// Start a listener on a port first
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind")
	}
	addr := ln.Addr().String()
	// Don't close ln — port is in use

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	err = ListenAndServeGraceful(addr, handler, time.Second)
	ln.Close()
	if err == nil {
		t.Error("expected error for port in use")
	}
}
