package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/chenyu/1-tok/internal/release"
)

const defaultDemoPrepareTimeout = 5 * time.Minute

func main() {
	timeout := readTimeout("RELEASE_DEMO_PREPARE_TIMEOUT_SECONDS", defaultDemoPrepareTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	summary, err := release.RunDemoPrepare(ctx, release.DemoRunConfigFromEnv())
	if writeErr := release.WriteJSONArtifact(os.Getenv("RELEASE_DEMO_PREPARE_OUTPUT_PATH"), summary); writeErr != nil {
		log.Fatal(writeErr)
	}
	if encodeErr := json.NewEncoder(os.Stdout).Encode(summary); encodeErr != nil {
		log.Fatal(encodeErr)
	}
	if err != nil {
		if errors.Is(err, release.ErrDemoNotReady) {
			log.Fatal(err)
		}
		log.Fatal(err)
	}
}

func readTimeout(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		log.Fatalf("invalid %s: %q", key, raw)
	}
	return time.Duration(seconds) * time.Second
}
