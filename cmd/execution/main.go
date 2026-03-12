package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/services/execution"
)

func main() {
	addr := envOrDefault("EXECUTION_ADDR", ":8085")
	shutdown, err := observability.InitFromEnv("execution")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	log.Printf("execution listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, observability.WrapHTTP("execution", execution.NewServer())))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
