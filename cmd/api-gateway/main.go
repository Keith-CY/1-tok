package main

import (
	"log"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
)

func main() {
	addr := envOrDefault("API_GATEWAY_ADDR", ":8080")
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
	log.Fatal(http.ListenAndServe(addr, gateway.NewServerWithApp(app)))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
