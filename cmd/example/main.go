package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/mongoprovider"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
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

	provider, ofClient, err := mongoprovider.New(mongoprovider.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
	if err != nil {
		log.Fatal("creating SingleDocumentProvider: ", err)
	}

	err = openfeature.SetProviderAndWait(provider)
	if err != nil {
		log.Fatal("setting provider: ", err)
	}
	client := openfeature.NewClient("sentry-client")

	v2Enbaled := client.String(context.TODO(), "v2_enabled", "provided_default", openfeature.NewEvaluationContext("1234", nil))
	fmt.Println("early v2_enabled:", v2Enbaled)

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

	time.Sleep(10 * time.Second)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.EvaluationContext{})
	fmt.Println("v2_enabled later:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("12345", map[string]any{}))
	fmt.Println("v2_enabled with user1:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("12345", map[string]any{
		"user_id": "12345",
	}))
	fmt.Println("v2_enabled with user2:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("123", map[string]any{
		"user_id": "123",
	}))
	fmt.Println("v2_enabled with user3:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("something", map[string]any{
		"unique_user_id": "something",
	}))
	fmt.Println("unique_user_id exists:", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("otherthing", map[string]any{
		"user_id": "otherthing",
	}))
	fmt.Println("fractional chance: ", v2Enbaled)

	v2Enbaled = client.String(context.TODO(), "v2_enabled", "second_static_default", openfeature.NewEvaluationContext("otherthing", map[string]any{
		"unique_id_thing":       "otherthing",
		"other_unique_id_thing": "otherthing",
	}))
	fmt.Println("and clause: ", v2Enbaled)

	time.Sleep(100 * time.Second)
}
