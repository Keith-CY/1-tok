package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/iam"
)

func main() {
	addr := envOrDefault("IAM_ADDR", ":8081")
	log.Printf("iam listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, iam.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
