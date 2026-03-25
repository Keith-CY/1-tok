package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/gateway"
	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/release"
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

	providerSettlementProvisioner, err := platform.NewFNNProviderSettlementProvisionerFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("api-gateway listening on %s", addr)
	gw, err := gateway.NewServerWithOptionsE(gateway.Options{
		App:                           app,
		ProviderSettlementProvisioner: providerSettlementProvisioner,
	})
	if err != nil {
		log.Fatal(err)
	}
	if os.Getenv("DEMO_AUTO_PREPARE") == "true" {
		go autoDemoPrepare(addr)
	}

	corsOrigin := envOrDefault("CORS_ALLOWED_ORIGIN", "*")
	handler := httputil.CORS(corsOrigin, httputil.LimitBody(gw, 0))
	if err := server.Run(addr, httputil.AccessLog("api-gateway", observability.WrapHTTP("api-gateway", handler)), 0); err != nil {
		log.Fatal(err)
	}
}

func autoDemoPrepare(listenAddr string) {
	cfg := release.DemoRunConfigFromEnv()
	healthURL := "http://localhost" + listenAddr + "/healthz"
	if !strings.Contains(listenAddr, ":") {
		healthURL = "http://localhost:" + listenAddr + "/healthz"
	}

	// Wait until the server is accepting connections.
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	summary, err := release.RunDemoPrepare(ctx, cfg)
	if err != nil {
		log.Printf("demo-auto-prepare: error: %v", err)
		return
	}
	log.Printf("demo-auto-prepare: ready (carrier=%s settlement=%s)",
		summary.Status.ProviderSettlement.CarrierBindingStatus,
		summary.Status.ProviderSettlement.SettlementBindingStatus)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
