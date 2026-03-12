package main

import (
	"log"
	"os"
	"time"

	"github.com/chenyu/1-tok/internal/bootstrap"
	"github.com/chenyu/1-tok/internal/observability"
)

func main() {
	shutdown, err := observability.InitFromEnv("bootstrap")
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown(2 * time.Second)

	if err := bootstrap.BootstrapDatabase(os.Getenv("DATABASE_URL")); err != nil {
		observability.CaptureError(nil, err)
		log.Fatal(err)
	}
	log.Printf("bootstrap completed")
}
