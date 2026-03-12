package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/chenyu/1-tok/internal/release"
)

func main() {
	summary, err := release.RunAbuseSmoke(context.Background(), release.AbuseConfigFromEnv())
	if err != nil {
		log.Fatal(err)
	}

	if outputPath := os.Getenv("RELEASE_ABUSE_OUTPUT_PATH"); outputPath != "" {
		if err := release.WriteJSONArtifact(outputPath, summary); err != nil {
			log.Fatal(err)
		}
	}

	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		log.Fatal(err)
	}
}
