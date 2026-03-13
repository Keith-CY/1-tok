package main

import (
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/server"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/services/iam"
)

func main() {
	addr := envOrDefault("IAM_ADDR", ":8081")
	shutdown, err := observability.InitFromEnv("iam")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("iam listening on %s", addr)
	handler := httputil.LimitBody(iam.NewServer(), 0)
	if err := server.Run(addr, httputil.AccessLog("iam", observability.WrapHTTP("iam", handler)), 0); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
