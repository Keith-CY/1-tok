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

const defaultDemoVerifyTimeout = 2 * time.Minute

func main() {
	timeout := readTimeout("RELEASE_DEMO_VERIFY_TIMEOUT_SECONDS", defaultDemoVerifyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	summary, err := release.RunDemoVerify(ctx, release.DemoRunConfigFromEnv())
	if writeErr := release.WriteJSONArtifact(os.Getenv("RELEASE_DEMO_VERIFY_OUTPUT_PATH"), summary); writeErr != nil {
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
