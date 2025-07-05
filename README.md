# OpenFeature MongoDB Provider (Go)

This repository is a MongoDB provider for the [OpenFeature Go SDK](https://openfeature.dev/docs/reference/technologies/server/go/).

## Features

-   Go OpenFeature SDK compatible MongoDB provider.
-   Supports both single-document and multi-document flag storage.
-   Provides a custom MongoDB client for managing flags.
-   Allows for flexible flag definitions with various rules.
-   Automatically watches changes in the MongoDB collection and updates flags accordingly.

## Usage

Install the provider:

```bash
go get github.com/zackarysantana/mongo-openfeature-go
```

Create the provider in your application:

```go
import (
    "github.com/open-feature/go-sdk/openfeature"
    "github.com/zackarysantana/mongo-openfeature-go/src/mongoprovider"
)
// ...

// Multi-document mode
provider, ofClient, err := mongoprovider.New(mongoprovider.NewOptions(mongoClient, database, collection))

// Single document mode
provider, ofClient, err := mongoprovider.New(mongoprovider.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
```

Set the provider in your application, and use the OpenFeature client:

```go
// Set the provider
err := openfeature.SetProviderAndWait(provider)

// Use the openfeature client.
client := openfeature.NewClient("sentry-client")
v2Enbaled := client.String(context.TODO(), "v2_enabled", "false", nil)
// For more usage on the client, refer to the OpenFeature Go SDK documentation.
```

The provider stores the flags in a particular way, so creating a provider also gives you a specialized client that can be used to update flags.

```go
import "github.com/zackarysantana/mongo-openfeature-go/src/flag"
// Some provider creation code
provider, ofClient, err := mongoprovider.New(mongoprovider.NewOptions(mongoClient, database, collection).WithDocumentID(documentID))
// ...

// Construct a flag definition with rules
flagDefinition := flag.Definition{
    FlagName: "v2_enabled_message",
    DefaultValue: "it's not enabled",
    DefaultVariant: "database_default",
    Rules: []rule.ConcreteRule{
        {ExactMatchRule: &rule.ExactMatchRule{
            Key:        "user_id",
            VariantID:  "superadmin-zackary",
            KeyValue:   "zackary_santana",
            ValueData:  "it's enabled for superadmin",
        }},
        {RegexRule: &rule.RegexRule{
            Key:           "user_email",
            VariantID:     "gmail-users",
            Pattern:       ".*@gmail.com",
            ValueData:     "it's enabled for gmail users",
        }},
    },
}

// Set the flag definition in the MongoDB provider
err := ofClient.SetFlag(context.TODO(), flagDefinition)
// Now future calls to the flag will use the new definition.
```

### Example

For a complete example, look at [cmd/example/main.go](cmd/example/main.go).
