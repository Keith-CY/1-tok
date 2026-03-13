package main

import (
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/server"
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
	gw, err := gateway.NewServerWithOptionsE(gateway.Options{
		App: app,
	})
	if err != nil {
		log.Fatal(err)
	}
	corsOrigin := envOrDefault("CORS_ALLOWED_ORIGIN", "*")
	handler := httputil.CORS(corsOrigin, httputil.LimitBody(gw, 0))
	if err := server.Run(addr, observability.WrapHTTP("api-gateway", handler), 0); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
