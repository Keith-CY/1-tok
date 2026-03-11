package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/settlement"
)

func main() {
	addr := envOrDefault("SETTLEMENT_ADDR", ":8083")
	log.Printf("settlement listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, settlement.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
