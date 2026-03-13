package main

import (
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/fiberadapter"
	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/server"
	"github.com/chenyu/1-tok/internal/observability"
)

func main() {
	addr := envOrDefault("FIBER_ADAPTER_ADDR", ":8091")
	shutdown, err := observability.InitFromEnv("fiber-adapter")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("fiber-adapter listening on %s", addr)
	handler := httputil.LimitBody(fiberadapter.NewServer(), 0)
	if err := server.Run(addr, httputil.AccessLog("fiber-adapter", observability.WrapHTTP("fiber-adapter", handler)), 0); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
