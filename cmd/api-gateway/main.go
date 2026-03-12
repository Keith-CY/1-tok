package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
	"github.com/chenyu/1-tok/internal/observability"
)

func main() {
	addr := envOrDefault("API_GATEWAY_ADDR", ":8080")
	shutdown, err := observability.InitFromEnv("api-gateway")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	app, cleanup, err := bootstrap.LoadPlatformApp()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Printf("api-gateway cleanup error: %v", err)
		}
	}()

	log.Printf("api-gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, observability.WrapHTTP("api-gateway", gateway.NewServerWithApp(app))))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
