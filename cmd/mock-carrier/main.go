package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/mockcarrier"
)

func main() {
	addr := envOrDefault("MOCK_CARRIER_ADDR", ":8787")
	log.Printf("mock-carrier listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mockcarrier.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
