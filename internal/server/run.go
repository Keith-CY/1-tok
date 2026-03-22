// Package server provides a shared HTTP server lifecycle with graceful shutdown.
package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DefaultDrainTimeout is the maximum time to wait for in-flight requests
// to complete after receiving a shutdown signal.
const DefaultDrainTimeout = 15 * time.Second

const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// Run starts an HTTP server on addr and blocks until a SIGINT or SIGTERM is
// received. In-flight requests are given up to drainTimeout to complete.
// Pass 0 for drainTimeout to use DefaultDrainTimeout.
func Run(addr string, handler http.Handler, drainTimeout time.Duration) error {
	if drainTimeout <= 0 {
		drainTimeout = DefaultDrainTimeout
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  envDurationSecondsOrDefault("SERVER_READ_TIMEOUT_SECONDS", defaultReadTimeout),
		WriteTimeout: envDurationSecondsOrDefault("SERVER_WRITE_TIMEOUT_SECONDS", defaultWriteTimeout),
		IdleTimeout:  envDurationSecondsOrDefault("SERVER_IDLE_TIMEOUT_SECONDS", defaultIdleTimeout),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		log.Printf("received %s, draining connections (timeout %s)", sig, drainTimeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()

	return srv.Shutdown(ctx)
}

func envDurationSecondsOrDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
