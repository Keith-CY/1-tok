package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/services/settlement"
)

func main() {
	addr := envOrDefault("SETTLEMENT_ADDR", ":8083")
	shutdown, err := observability.InitFromEnv("settlement")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("settlement listening on %s", addr)
	handler := httputil.LimitBody(settlement.NewServer(), 0)
	log.Fatal(http.ListenAndServe(addr, observability.WrapHTTP("settlement", handler)))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
