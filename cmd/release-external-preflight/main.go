package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/release"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cfg := release.ExternalDependencyConfigFromEnv()
	if err := release.RunExternalDependencyPreflight(ctx, cfg); err != nil {
		log.Fatal(err)
	}
	if err := release.WriteJSONArtifact(os.Getenv("RELEASE_EXTERNAL_PREFLIGHT_OUTPUT_PATH"), map[string]any{
		"status":  "ok",
		"fiber":   cfg.FiberRPCURL,
		"carrier": cfg.CarrierGatewayURL,
	}); err != nil {
		log.Fatal(err)
	}
	log.Printf("external dependency preflight completed")
}
