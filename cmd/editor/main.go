package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zackarysantana/mongo-openfeature-go/cmd/internal"
	"github.com/zackarysantana/mongo-openfeature-go/internal/editor"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

func main() {
	mongoClient, ofClient, cleanup, err := internal.GetConnections()
	if err != nil {
		log.Fatalf("FATAL: getting connections: %v", err)
	}
	defer cleanup()

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
		return fmt.Errorf("inserting example feature flag: %w", err)
	}

	return nil
}
