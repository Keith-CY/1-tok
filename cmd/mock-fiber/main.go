package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/mockfiber"
)

func main() {
	addr := envOrDefault("MOCK_FIBER_ADDR", ":8090")
	log.Printf("mock-fiber listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mockfiber.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
