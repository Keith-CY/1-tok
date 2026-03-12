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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	summary, err := release.RunFNNDualNodeSmoke(ctx, release.FNNDualNodeSmokeConfigFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	if err := release.WriteJSONArtifact(os.Getenv("RELEASE_FNN_DUAL_OUTPUT_PATH"), summary); err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		log.Fatal(err)
	}
}
