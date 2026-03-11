package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/notification"
)

func main() {
	addr := envOrDefault("NOTIFICATION_ADDR", ":8086")
	log.Printf("notification listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, notification.NewServer()))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
