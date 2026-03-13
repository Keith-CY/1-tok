package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/services/marketplace"
)

func main() {
	addr := envOrDefault("MARKETPLACE_ADDR", ":8082")
	shutdown, err := observability.InitFromEnv("marketplace")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("marketplace listening on %s", addr)
	handler := httputil.LimitBody(marketplace.NewServer(), 0)
	log.Fatal(http.ListenAndServe(addr, observability.WrapHTTP("marketplace", handler)))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
