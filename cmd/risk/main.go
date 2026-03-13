package main

import (
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/server"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/services/risk"
)

func main() {
	addr := envOrDefault("RISK_ADDR", ":8084")
	shutdown, err := observability.InitFromEnv("risk")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("risk listening on %s", addr)
	handler := httputil.LimitBody(risk.NewServer(), 0)
	if err := server.Run(addr, observability.WrapHTTP("risk", handler), 0); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
