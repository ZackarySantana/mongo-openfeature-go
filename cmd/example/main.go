package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/editor"
	"github.com/zackarysantana/mongo-openfeature-go/internal/testutil"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/mongoprovider"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func main() {
	database := "openfeature"
	collection := "feature_flags"
	documentID := "feature_flags"

	cleanup, err := testutil.CreateMongoContainer(context.Background())
	if err != nil {
		log.Fatalf("FATAL: creating MongoDB container: %v", err)
	}
	defer cleanup()
	mongoURI := os.Getenv("MONGODB_ENDPOINT")

	log.Println("Connecting to TestContainer Mongo at:", mongoURI)

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("FATAL: connecting to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	provider, ofClient, err := mongoprovider.New(mongoprovider.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
	if err != nil {
		log.Fatalf("FATAL: creating MongoDB OpenFeature provider: %v", err)
	}

	if err := openfeature.SetNamedProvider("client", provider); err != nil {
		log.Fatalf("FATAL: setting OpenFeature provider: %v", err)
	}

	client := openfeature.NewClient("client")

	go func() {
		for {
			time.Sleep(5 * time.Second)
			fmt.Println("")

			user1V2Enabled := client.String(context.TODO(), "v2_enabled", "static", openfeature.NewEvaluationContext("user_id", map[string]any{
				"user_id":    "1",
				"role":       "admin",
				"region":     "us-west",
				"ip_address": "192.168.192.0",
				"time":       time.Now(),
			}))
			fmt.Println("user_1: ", user1V2Enabled)
			user2V2Enabled := client.String(context.TODO(), "v2_enabled", "static", openfeature.NewEvaluationContext("user_id", map[string]any{
				"user_id":    "2",
				"role":       "user",
				"region":     "us-east",
				"ip_address": "192.168.168.0",
				"time":       time.Now().Add(24 * time.Hour * 365), // 1 year later
			}))
			fmt.Println("user_2: ", user2V2Enabled)
			userFooV2Enabled := client.String(context.TODO(), "v2_enabled", "static", openfeature.NewEvaluationContext("user_id", map[string]any{
				"user_id":             "foo",
				"role":                "guest",
				"region":              "europe",
				"ip_address":          "102.168.55.0",
				"unique_user_field":   "some_value",
				"second_unique_field": "another_value",
			}))
			fmt.Println("user_foo: ", userFooV2Enabled)
			userBarV2Enabled := client.String(context.TODO(), "v2_enabled", "static", openfeature.NewEvaluationContext("user_id", map[string]any{
				"user_id":             "bar",
				"ip_address":          "102.168.55.0",
				"second_unique_field": "another_value",
			}))
			fmt.Println("user_foo: ", userBarV2Enabled)
		}
	}()

	if err = insertExampleData(ofClient); err != nil {
		log.Fatalf("FATAL: inserting example data: %v", err)
	}

	if err = editor.RunEditor(mongoClient, ofClient); err != nil {
		log.Fatalf("FATAL: starting editor server: %v", err)
	}
}

// insertExampleData inserts example feature flag data into the MongoDB OpenFeature client.
func insertExampleData(ofClient *client.Client) error {
	flagDefinition := flag.Definition{
		FlagName:       "v2_enabled",
		DefaultValue:   "false",
		DefaultVariant: "database_default",
		Rules: []rule.ConcreteRule{
			{ExactMatchRule: &rule.ExactMatchRule{
				Key:       "user_id",
				VariantID: "exact-rule",
				KeyValue:  "1",
				ValueData: "exact-id-1",
			}},
			{RegexRule: &rule.RegexRule{
				Key:       "user_id",
				VariantID: "regex-rule",
				Pattern:   "^[0-9]$",
				ValueData: "regex-id-is-number",
			}},
			{ExistsRule: &rule.ExistsRule{
				Key:       "unique_user_field",
				VariantID: "exists-rule",
				ValueData: "exists-unique-field",
			}},
			{IPRangeRule: &rule.IPRangeRule{
				Key:       "ip_address",
				VariantID: "ip-range-rule",
				CIDRs:     []string{"192.168.0.0/16"},
				ValueData: "ip-range-192-168-0-0-16",
			}},
			{PrefixRule: &rule.PrefixRule{
				Key:       "region",
				VariantID: "prefix-rule",
				Prefix:    "us-",
				ValueData: "prefix-us-",
			}},
			{SuffixRule: &rule.SuffixRule{
				Key:       "role",
				VariantID: "suffix-rule",
				Suffix:    "-admin",
				ValueData: "suffix-role-admin",
			}},
			{DateTimeRule: &rule.DateTimeRule{
				Key:       "time",
				VariantID: "datetime-rule",
				After:     time.Now().Add(-24 * time.Hour), // 24 hours ago
				Before:    time.Now().Add(24 * time.Hour),  // 24 hours from now
				ValueData: "datetime-within-24-hours",
			}},
			{FractionalRule: &rule.FractionalRule{
				Key:        "user_id",
				VariantID:  "fractional-rule",
				Percentage: 50.0,
				ValueData:  "fractional-50-percent",
			}},
			{AndRule: &rule.AndRule{
				Rules: []rule.ConcreteRule{
					{ExistsRule: &rule.ExistsRule{
						Key: "second_unique_field",
					}},
					{ExactMatchRule: &rule.ExactMatchRule{
						Key:      "ip_address",
						KeyValue: "102.168.55.0",
					}},
				},
				ValueData: "and-rule-unique-and-ip",
			}},
			{OverrideRule: &rule.OverrideRule{
				VariantID: "override-rule",
				ValueData: "override-value",
			}},
		},
	}

	if err := ofClient.SetFlag(context.TODO(), flagDefinition); err != nil {
		log.Fatal("updating feature flags: ", err)
	}

	return nil
}
