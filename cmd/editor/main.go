package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/editor"
	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/mongoprovider"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	database   = "example_db"
	collection = "flags" // The UI will manage the 'flags' collection
	documentID = "feature_flags"
)

func main() {
	// 1. Start an ephemeral MongoDB container for the application to use.
	// This is taken directly from your example.
	cleanup, err := testutil.CreateMongoContainer(context.Background())
	if err != nil {
		log.Fatalf("FATAL: creating MongoDB container: %v", err)
	}
	defer cleanup()

	fmt.Println("MongoDB test container endpoint: ", os.Getenv("MONGODB_ENDPOINT"))

	// 2. Connect a mongo.Client to the test container.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(os.Getenv("MONGODB_ENDPOINT")))
	if err != nil {
		log.Fatalf("FATAL: connecting to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(ctx)

	// 3. Run the main application logic with the connected client.
	if err := run(ctx, mongoClient); err != nil {
		log.Fatalf("FATAL: Application failed to run: %v", err)
	}
}

// run configures and starts the web application.
func run(ctx context.Context, mongoClient *mongo.Client) error {
	provider, ofClient, err := mongoprovider.New(
		mongoprovider.NewOptions(mongoClient, database, collection).WithDocumentID(documentID),
	)
	if err != nil {
		return fmt.Errorf("creating OpenFeature provider: %w", err)
	}
	if err := openfeature.SetProviderAndWait(provider); err != nil {
		return fmt.Errorf("setting OpenFeature provider: %w", err)
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
			// {OverrideRule: &rule.OverrideRule{
			// 	VariantID: "override-rule",
			// 	ValueData: "override_default",
			// }},
		},
	}

	fmt.Println("Updating the feature flags")
	if err = ofClient.SetFlag(context.TODO(), flagDefinition); err != nil {
		log.Fatal("updating feature flags: ", err)
	}

	log.Println("OpenFeature provider initialized and set.")

	// 3. Initialize the web handlers, passing in the client they will use.
	handler := editor.NewWebHandler(ofClient)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /editor.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/editor/editor.js")
	})

	// Your existing routes
	mux.HandleFunc("GET /edit/", handler.HandleEditFlag)
	mux.HandleFunc("GET /edit/{name}", handler.HandleEditFlag)
	mux.HandleFunc("POST /save", handler.HandleSaveFlag)
	mux.HandleFunc("POST /delete", handler.HandleDeleteFlag)
	mux.HandleFunc("GET /", handler.HandleListFlags)

	// --- Start Server ---
	log.Println("Starting flag manager UI on http://localhost:8080")
	return http.ListenAndServe(":8080", mux)
}
