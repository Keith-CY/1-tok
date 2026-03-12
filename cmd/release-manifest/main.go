package main

import (
	"log"
	"os"
	"strings"

	"github.com/chenyu/1-tok/internal/release"
)

func main() {
	outputPath := strings.TrimSpace(os.Getenv("RELEASE_MANIFEST_OUTPUT_PATH"))
	if outputPath == "" {
		log.Fatal("RELEASE_MANIFEST_OUTPUT_PATH is required")
	}

	if err := release.WriteReleaseManifest(outputPath, release.ReleaseManifestInput{
		ArtifactDir:       strings.TrimSpace(os.Getenv("RELEASE_ARTIFACT_DIR")),
		GitSHA:            strings.TrimSpace(os.Getenv("RELEASE_GIT_SHA")),
		FiberRPCURL:       strings.TrimSpace(os.Getenv("DEPENDENCY_FIBER_RPC_URL")),
		CarrierGatewayURL: strings.TrimSpace(os.Getenv("DEPENDENCY_CARRIER_GATEWAY_URL")),
	}); err != nil {
		log.Fatal(err)
	}

	log.Printf("release manifest written to %s", outputPath)
}
