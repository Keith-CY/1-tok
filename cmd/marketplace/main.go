package main

import (
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/server"
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
	if err := server.Run(addr, httputil.AccessLog("marketplace", observability.WrapHTTP("marketplace", handler)), 0); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
