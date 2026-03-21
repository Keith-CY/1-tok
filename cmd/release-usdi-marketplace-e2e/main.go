package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/release"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	summary, err := release.RunUSDIMarketplaceE2E(ctx, release.USDIMarketplaceE2EConfigFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	if err := release.WriteJSONArtifact(os.Getenv("RELEASE_USDI_E2E_OUTPUT_PATH"), summary); err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		log.Fatal(err)
	}
}
