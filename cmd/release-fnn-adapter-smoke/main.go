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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	summary, err := release.RunFNNAdapterSmoke(ctx, release.FNNAdapterSmokeConfigFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	if err := release.WriteJSONArtifact(os.Getenv("RELEASE_FNN_ADAPTER_OUTPUT_PATH"), summary); err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		log.Fatal(err)
	}
}
