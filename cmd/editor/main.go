package main

import (
	"context"
	"log"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/internal/editor"
	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	database := os.Getenv("MONGODB_DATABASE")
	if database == "" {
		database = "openfeature"
	}
	collection := os.Getenv("MONGODB_COLLECTION")
	if collection == "" {
		collection = "feature_flags"
	}
	documentID := os.Getenv("MONGODB_DOCUMENT_ID")
	if documentID == "" {
		documentID = "feature_flags"
	}

	if os.Getenv("USE_TESTCONTAINER") == "true" {
		log.Println("USE_TESTCONTAINER is true, starting MongoDB container...")
		cleanup, err := testutil.CreateMongoContainer(context.Background())
		if err != nil {
			log.Fatalf("FATAL: creating MongoDB container: %v", err)
		}
		defer cleanup()

		log.Println("Connecting to TestContainer Mongo at:", os.Getenv("MONGODB_ENDPOINT"))
	}
	mongoURI := os.Getenv("MONGODB_ENDPOINT")
	if mongoURI == "" {
		log.Fatalf("FATAL: MONGODB_ENDPOINT environment variable is not set and USE_TESTCONTAINER is not true.")
	}

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("FATAL: connecting to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	ofClient, err := client.New(client.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
	if err != nil {
		log.Fatalf("FATAL: creating MongoDB OpenFeature client: %v", err)
	}

	if err = insertExampleData(ofClient); err != nil {
		log.Fatalf("FATAL: inserting example data: %v", err)
	}

	if err = editor.RunEditor(mongoClient, ofClient); err != nil {
		log.Fatalf("FATAL: starting editor server: %v", err)
	}
}

// insertExampleData inserts example feature flag data into the MongoDB OpenFeature client.
func insertExampleData(ofClient *client.Client) error {
	if os.Getenv("USE_TESTCONTAINER") != "true" {
		return nil
	}

	flagDefinition := flag.Definition{
		FlagName:       "v2_enabled",
		DefaultValue:   "false",
		DefaultVariant: "database_default",
		Rules: []rule.ConcreteRule{
			{ExactMatchRule: &rule.ExactMatchRule{
				Key:       "user_id",
				VariantID: "exact-rule",
				KeyValue:  "12345",
				ValueData: "database_default_2",
			}},
			{RegexRule: &rule.RegexRule{
				Key:       "user_id",
				VariantID: "regex-rule",
				Pattern:   "^[0-9]{3}$",
				ValueData: "regex_default",
			}},
			{ExistsRule: &rule.ExistsRule{
				Key:       "unique_user_id",
				VariantID: "exists-rule",
				ValueData: "exists_default",
			}},
			{FractionalRule: &rule.FractionalRule{
				Key:        "user_id",
				VariantID:  "fractional-rule",
				Percentage: 1.0,
				ValueData:  "fractional_default_small",
			}},
			{FractionalRule: &rule.FractionalRule{
				Key:        "user_id",
				VariantID:  "fractional-rule",
				Percentage: 99.0,
				ValueData:  "fractional_default_large",
			}},
			{AndRule: &rule.AndRule{
				Rules: []rule.ConcreteRule{
					{ExistsRule: &rule.ExistsRule{
						Key: "unique_id_thing",
					}},
					{ExistsRule: &rule.ExistsRule{
						Key: "other_unique_id_thing",
					}},
				},
				ValueData: "and_default",
			}},
			{OverrideRule: &rule.OverrideRule{
				VariantID: "override-rule",
				ValueData: "override_default",
			}},
		},
	}

	if err := ofClient.SetFlag(context.TODO(), flagDefinition); err != nil {
		log.Fatal("updating feature flags: ", err)
	}

	return nil
}
