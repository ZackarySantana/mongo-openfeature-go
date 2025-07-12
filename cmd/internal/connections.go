package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func GetMongoDatabaseName() string {
	if database := os.Getenv("MONGODB_DATABASE"); database != "" {
		return database
	}
	return "openfeature"
}

func GetMongoCollectionName() string {
	if collection := os.Getenv("MONGODB_COLLECTION"); collection != "" {
		return collection
	}
	return "feature_flags"
}

func GetMongoDocumentID() string {
	if documentID, found := os.LookupEnv("MONGODB_DOCUMENT_ID"); found {
		return documentID
	}
	return "feature_flags"
}

func GetConnections() (*mongo.Client, *client.Client, func(), error) {
	database := GetMongoDatabaseName()
	collection := GetMongoCollectionName()
	documentID := GetMongoDocumentID()

	cleanup := func() {}

	if os.Getenv("USE_TESTCONTAINER") == "true" {
		var err error
		cleanup, err = testutil.CreateMongoContainer(context.Background())
		if err != nil {
			return nil, nil, nil, err
		}

		log.Println("Connecting to TestContainer Mongo at:", os.Getenv("MONGODB_ENDPOINT"))
	}

	mongoURI := os.Getenv("MONGODB_ENDPOINT")
	if mongoURI == "" {
		cleanup()
		return nil, nil, nil, errors.New("MONGODB_ENDPOINT environment variable is not set")
	}

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("connecting to MongoDB: %w", err)
	}
	cleanup = func() {
		cleanup()
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("Error disconnecting MongoDB client: %v", err)
		}
	}

	ofClient, err := client.New(client.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("creating MongoDB OpenFeature client: %w", err)
	}

	return mongoClient, ofClient, cleanup, nil
}
