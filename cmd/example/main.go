package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/rules"
	"github.com/zackarysantana/mongo-openfeature-go/src/singledocument"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	database   = "example_db"
	collection = "example_collection"
	documentID = "feature_flags"
)

func main() {
	cleanup, err := testutil.CreateMongoContainer(context.TODO())
	if err != nil {
		log.Fatal("creating MongoDB container: ", err)
	}
	defer cleanup()

	fmt.Println("MongoDB endpoint: ", os.Getenv("MONGODB_ENDPOINT"))
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(os.Getenv("MONGODB_ENDPOINT")))
	if err != nil {
		log.Fatal("connecting to MongoDB: ", err)
	}

	provider, err := singledocument.NewSingleDocumentProvider(&singledocument.SingleDocumentProviderOptions{
		Client:     mongoClient,
		Database:   database,
		Collection: collection,
		DocumentID: documentID,
	})
	if err != nil {
		log.Fatal("creating SingleDocumentProvider: ", err)
	}

	err = openfeature.SetProviderAndWait(provider)
	if err != nil {
		log.Fatal("setting provider: ", err)
	}
	client := openfeature.NewClient("sentry-client")

	v2Enbaled := client.String(context.TODO(), "v2_enabled", "provided_default", openfeature.EvaluationContext{})
	fmt.Println("early v2_enabled:", v2Enbaled)

	go func() {
		time.Sleep(1 * time.Second)
		mongoClient, err := mongo.Connect(options.Client().ApplyURI(os.Getenv("MONGODB_ENDPOINT")))
		if err != nil {
			log.Fatal("connecting to MongoDB: ", err)
		}

		flagDefinition := rules.FlagDefinition{
			FlagName:       "v2_enabled",
			DefaultValue:   "false",
			DefaultVariant: "database_default",
			Rules: []rules.ConcreteRule{{
				ExactMatchRule: &rules.ExactMatchRule{
					Key:       "user_id",
					VariantID: "exact-rule",
					KeyValue:  "12345",
					ValueData: "database_default_2",
				}},
			},
		}

		document := map[string]rules.FlagDefinition{
			"v2_enabled": flagDefinition,
		}

		fmt.Println("Updating the feature flags")
		result, err := mongoClient.Database(database).Collection(collection).UpdateByID(
			context.TODO(),
			documentID,
			bson.M{
				"$set": document,
			},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			log.Fatal("updating feature flags: ", err)
		}
		fmt.Println("Update result:", result)
	}()

	time.Sleep(10 * time.Second)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.EvaluationContext{})
	fmt.Println("v2_enabled later:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("12345", map[string]any{}))
	fmt.Println("v2_enabled with user1:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("12345", map[string]any{
		"user_id": "12345",
	}))
	fmt.Println("v2_enabled with user2:", v2Enbaled)

	time.Sleep(100 * time.Second)
}
