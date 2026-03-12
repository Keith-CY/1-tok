package main

import (
	"context"
	"log"
	"time"

	"github.com/chenyu/1-tok/internal/release"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := release.RunExternalDependencyPreflight(ctx, release.ExternalDependencyConfigFromEnv()); err != nil {
		log.Fatal(err)
	}
	log.Printf("external dependency preflight completed")
}
