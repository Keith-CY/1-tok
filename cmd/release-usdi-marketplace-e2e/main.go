package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/chenyu/1-tok/internal/release"
)

const defaultUSDIMarketplaceE2ETimeout = 10 * time.Minute

func main() {
	timeout := defaultUSDIMarketplaceE2ETimeout
	if raw := os.Getenv("RELEASE_USDI_E2E_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			log.Fatalf("invalid RELEASE_USDI_E2E_TIMEOUT_SECONDS: %q", raw)
		}
		timeout = time.Duration(seconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
