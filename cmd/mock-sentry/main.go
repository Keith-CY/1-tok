package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/mocksentry"
)

func main() {
	addr := envOrDefault("MOCK_SENTRY_ADDR", ":8092")
	log.Printf("mock-sentry listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mocksentry.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
