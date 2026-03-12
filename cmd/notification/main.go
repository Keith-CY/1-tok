package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/services/notification"
)

func main() {
	addr := envOrDefault("NOTIFICATION_ADDR", ":8086")
	shutdown, err := observability.InitFromEnv("notification")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("notification listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, observability.WrapHTTP("notification", notification.NewServer())))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
