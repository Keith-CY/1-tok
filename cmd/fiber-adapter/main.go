package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/fiberadapter"
)

func main() {
	addr := envOrDefault("FIBER_ADAPTER_ADDR", ":8091")
	log.Printf("fiber-adapter listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, fiberadapter.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
