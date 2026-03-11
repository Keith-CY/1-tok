package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/risk"
)

func main() {
	addr := envOrDefault("RISK_ADDR", ":8084")
	log.Printf("risk listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, risk.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
