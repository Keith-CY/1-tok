package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/fiberadapter"
	"github.com/chenyu/1-tok/internal/httputil"
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
	log.Fatal(http.ListenAndServe(addr, observability.WrapHTTP("fiber-adapter", handler)))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
