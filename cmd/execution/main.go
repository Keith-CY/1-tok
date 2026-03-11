package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/execution"
)

func main() {
	addr := envOrDefault("EXECUTION_ADDR", ":8085")
	log.Printf("execution listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, execution.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
