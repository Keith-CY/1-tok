package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/gateway"
)

func main() {
	addr := envOrDefault("API_GATEWAY_ADDR", ":8080")
	log.Printf("api-gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, gateway.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
