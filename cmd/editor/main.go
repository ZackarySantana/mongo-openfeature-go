package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/internal/editor"
	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	database   = "feature_flags"
	collection = "feature_flags"
	documentID = ""
)

func main() {
	var mongoURI string

	if os.Getenv("USE_TESTCONTAINER") == "true" {
		log.Println("USE_TESTCONTAINER is true, starting MongoDB container...")
		cleanup, err := testutil.CreateMongoContainer(context.Background())
		if err != nil {
			log.Fatalf("FATAL: creating MongoDB container: %v", err)
		}
		defer cleanup()
		mongoURI = os.Getenv("MONGODB_ENDPOINT")
		log.Println("Connecting to TestContainer Mongo at:", mongoURI)
	} else {
		log.Println("Using external MongoDB. Set MONGODB_URI to configure.")
		mongoURI = os.Getenv("MONGODB_URI")
		if mongoURI == "" {
			log.Fatalf("FATAL: MONGODB_URI environment variable is not set and USE_TESTCONTAINER is not true.")
		}
	}

	if db := os.Getenv("MONGODB_DATABASE"); db != "" {
		database = db
	}
	if coll := os.Getenv("MONGODB_COLLECTION"); coll != "" {
		collection = coll
	}
	if id := os.Getenv("MONGODB_DOCUMENT_ID"); id != "" {
		documentID = id
	}

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("FATAL: connecting to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	// 3. Run the main application logic with the connected client.
	if err := run(mongoClient); err != nil {
		log.Fatalf("FATAL: Application failed to run: %v", err)
	}
}

// run configures and starts the web application.
func run(mongoClient *mongo.Client) error {
	ofClient, err := client.New(client.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
	if err != nil {
		return fmt.Errorf("creating MongoDB OpenFeature client: %w", err)
	}

	if os.Getenv("USE_TESTCONTAINER") == "true" {
		// Seed an example feature flag for testing purposes.
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

		if err = ofClient.SetFlag(context.TODO(), flagDefinition); err != nil {
			log.Fatal("updating feature flags: ", err)
		}
	}

	handler := editor.NewWebHandler(ofClient)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /editor.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/editor/editor.js")
	})

	mux.HandleFunc("GET /edit/", handler.HandleEditFlag)
	mux.HandleFunc("GET /edit/{name}", handler.HandleEditFlag)
	mux.HandleFunc("POST /save", handler.HandleSaveFlag)
	mux.HandleFunc("POST /delete", handler.HandleDeleteFlag)
	mux.HandleFunc("GET /", handler.HandleListFlags)

	log.Println("Starting flag manager UI on http://localhost:8080")
	return http.ListenAndServe(":8080", mux)
}
