package main

import (
	"log"
	"os"

	"github.com/chenyu/1-tok/internal/bootstrap"
)

func main() {
	if err := bootstrap.BootstrapDatabase(os.Getenv("DATABASE_URL")); err != nil {
		log.Fatal(err)
	}
	log.Printf("bootstrap completed")
}
