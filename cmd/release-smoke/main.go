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

	summary, err := release.RunSmoke(ctx, release.ConfigFromEnv())
	if err != nil {
		log.Fatal(err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(summary); err != nil {
		log.Fatal(err)
	}
}
