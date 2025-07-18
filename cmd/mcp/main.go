package main

import (
	"log"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/cmd/internal"
	"github.com/zackarysantana/mongo-openfeature-go/internal/mcp"
)

func main() {
	mcpServe := os.Getenv("MCP_SERVE")
	mongoClient, ofClient, cleanup, err := internal.GetConnections(mcpServe == "http" || mcpServe == "sse")
	if err != nil {
		log.Fatalf("FATAL: getting connections: %v", err)
	}
	defer cleanup()

	if os.Getenv("USE_TESTCONTAINER") == "true" {
		if err = internal.InsertExampleData(ofClient); err != nil {
			log.Fatalf("FATAL: inserting example data: %v", err)
		}
	}

	database := internal.GetMongoDatabaseName()
	collection := internal.GetMongoCollectionName()
	documentID := internal.GetMongoDocumentID()

	if err = mcp.Serve(mongoClient.Database(database).Collection(collection), ofClient, documentID); err != nil {
		log.Fatalf("FATAL: starting MCP server: %v", err)
	}
}
