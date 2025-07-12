package main

import (
	"log"

	"github.com/zackarysantana/mongo-openfeature-go/cmd/internal"
	"github.com/zackarysantana/mongo-openfeature-go/internal/mcp"
)

func main() {
	mongoClient, ofClient, cleanup, err := internal.GetConnections()
	if err != nil {
		log.Fatalf("FATAL: getting connections: %v", err)
	}
	defer cleanup()

	database := internal.GetMongoDatabaseName()
	collection := internal.GetMongoCollectionName()
	documentID := internal.GetMongoDocumentID()

	if err = mcp.Serve(mongoClient.Database(database).Collection(collection), ofClient, documentID); err != nil {
		log.Fatalf("FATAL: starting MCP server: %v", err)
	}
}
