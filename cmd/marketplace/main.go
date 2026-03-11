package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/marketplace"
)

func main() {
	addr := envOrDefault("MARKETPLACE_ADDR", ":8082")
	log.Printf("marketplace listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, marketplace.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
