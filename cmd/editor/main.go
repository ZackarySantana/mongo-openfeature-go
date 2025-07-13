package main

import (
	"log"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/cmd/internal"
	"github.com/zackarysantana/mongo-openfeature-go/internal/editor"
)

func main() {
	mongoClient, ofClient, cleanup, err := internal.GetConnections(true)
	if err != nil {
		log.Fatalf("FATAL: getting connections: %v", err)
	}
	defer cleanup()

	if os.Getenv("USE_TESTCONTAINER") == "true" {
		if err = internal.InsertExampleData(ofClient); err != nil {
			log.Fatalf("FATAL: inserting example data: %v", err)
		}
	}

	if err = editor.RunEditor(mongoClient, ofClient); err != nil {
		log.Fatalf("FATAL: starting editor server: %v", err)
	}
}
