# OpenFeature MongoDB Provider (Go)

This repository is a MongoDB provider for the [OpenFeature Go SDK](https://openfeature.dev/docs/reference/technologies/server/go/).

-   [Features](#features)
-   [Usage](#usage)
-   [Standard Rules](#standard-rules)
-   [Control Rules](#control-rules)
-   [Example](#example)
-   [Editor](#editor)

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
provider, ofClient, err := mongoprovider.New(
    mongoprovider.NewOptions(mongoClient, database, collection),
)

// Single document mode
provider, ofClient, err := mongoprovider.New(
    mongoprovider.NewOptions(mongoClient, database, collection).
        WithDocumentID(documentID),
)
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
provider, ofClient, err := mongoprovider.New(
    mongoprovider.NewOptions(mongoClient, database, collection).
        WithDocumentID(documentID),
)
// ...

// Construct a flag definition with rules
flagDefinition := flag.Definition{
    FlagName: "v2_enabled_message",
    DefaultValue: "it's not enabled",
    DefaultVariant: "database_default",
    Rules: []rule.ConcreteRule{
        {ExactMatchRule: &rule.ExactMatchRule{
            Key:        "user_id",
            KeyValue:   "zackary_santana",
            VariantID:  "superadmin-zackary",
            ValueData:  "it's enabled for superadmin",
        }},
        {RegexRule: &rule.RegexRule{
            Key:           "user_email",
            Pattern:       ".*@gmail.com",
            VariantID:     "gmail-users",
            ValueData:     "it's enabled for gmail users",
        }},
    },
}

// Set the flag definition in the MongoDB provider
err := ofClient.SetFlag(context.TODO(), flagDefinition)
// Now future calls to the flag will use the new definition.
```

### Standard Rules

Rules share common values, like Key, VariantID, Priority, and ValueData. For example, this ExactMatchRule:

```go
rule.ExactMatchRule{
    Key:        "user_id",
    KeyValue:   "zackary_santana",
    VariantID:  "superadmin-zackary",
    Priority: 100,
    ValueData:  "it's enabled for superadmin",
}
```

Matches on the key 'user_id'. If it's matched, it will inform the OpenFeature SDK of it's 'VariantID' and 'ValueData'. It also has `Priority` of 100, which is used to determine which rule supercedes others when multiple rules match. Higher priority rules will take precedence over lower priority ones. The only exception is the [OverrideRule](#overriderule), which is documented under that rule.

The list of standard rules includes:

-   [ExactMatchRule](#exactmatchrule)
-   [RegexRule](#regexrule)
-   [ExistsRule](#existsrule)
-   [FractionalRule](#fractionalrule)
-   [RangeRule](#rangerule)
-   [InListRule](#inlistrule)
-   [PrefixRule](#prefixrule)
-   [SuffixRule](#suffixrule)
-   [ContainsRule](#containsrule)
-   [IPRangeRule](#iprangerule)
-   [GeoFenceRule](#geofencerule)
-   [DateTimeRule](#datetimerule)
-   [SemVerRule](#semverrule)
-   [CronRule](#cronrule)

There are also [control rules](#control-rules) that can be used to combine, negate, or override other rules:

#### ExactMatchRule

```go
ExactMatchRule: &rule.ExactMatchRule{
    Key:        "user_id",
    KeyValue:   "zackary_santana",
    VariantID:  "superadmin-zackary",
    ValueData:  "allow_all",
}
```

Matches 'user_id' exactly with 'zackary_santana'.

#### RegexRule

```go
RegexRule: &rule.RegexRule{
    Key:           "user_email",
    Pattern:       ".*@gmail.com",
    VariantID:     "gmail-users",
    ValueData:     true,
}
```

Matches 'user_email' with a regex pattern (using golang's std regex package).

#### ExistsRule

```go
ExistsRule: &rule.ExistsRule{
    Key:        "verified",
    VariantID:  "exists-rule",
    ValueData:  "use_verified",
}
```

Matches if the key 'verified' exists in the context.

#### FractionalRule

```go
FractionalRule: &rule.FractionalRule{
    Key:        "user_id",
    Percentage: 10,
    VariantID:  "fractional-rule",
    ValueData:  "beta_test",
}
```

Matches if the key 'user_id' is in the top 10% of users. It uses a hash of the key + the key's value. For example, a user_id of 'zackary_santana' would get hashed by user_idzackary_santana, and if the hash is less than 10% of the total hash space, it will match.

#### RangeRule

```go
RangeRule: &rule.RangeRule{
    Key:        "user_age",
    Min:        18,
    Max:        99,
    ExlusiveMin: false,
    ExclusiveMax: true,
    VariantID:  "age-range",
    ValueData:  "adult_user",
}
```

Matches if the key 'user_age' is between 18 and 99, inclusive of 18 but exclusive of 99. Omitting `ExclusiveMin` or `ExclusiveMax` will default to `false`, meaning the range is inclusive.

#### InListRule

```go
InListRule: &rule.InListRule{
    Key:        "user_role",
    Values:     []string{"admin", "superadmin", "moderator"},
    VariantID:  "in-list-rule",
    ValueData:  "has_special_role",
}
```

Matches if the key 'user_role' is in the list of values provided, which is compared by `==` or `reflect.DeepEqual`. The list can contain any number of values.

#### PrefixRule

```go
PrefixRule: &rule.PrefixRule{
    Key:        "user_id",
    Prefix:     "zackary_",
    VariantID:  "prefix-rule",
    ValueData:  "zackary_user",
}
```

Matches if the key `user_id` starts with the prefix `zackary_`.

#### SuffixRule

```go
SuffixRule: &rule.SuffixRule{
    Key:        "user_id",
    Suffix:     "_santana",
    VariantID:  "suffix-rule",
    ValueData:  "santana_user",
}
```

Matches if the key `user_id` ends with the suffix `_santana`.

#### ContainsRule

```go
ContainsRule: &rule.ContainsRule{
    Key:        "user_id",
    Substring:  "zackary",
    VariantID:  "contains-rule",
    ValueData:  "contains_zackary",
}
```

Matches if the key `user_id` contains the substring `zackary`.

#### IPRangeRule

```go
IPRangeRule: &rule.IPRangeRule{
    Key:        "user_ip",
    CIDR:       []string{"192.168.1.1", "192.168.1.0"},
    VariantID:  "ip-range-rule",
    ValueData:  "ip_in_range",
}
```

Matches if the key `user_ip` is within the specified CIDR range. The CIDR can be a single IP or a range in CIDR notation (for example, `192.168.1.0/24` would be considered in range because of the 2nd CIDR in the rule).

#### GeoFenceRule

```go
GeoFenceRule: &rule.GeoFenceRule{
    Key:        "user_location",
    LatKey:       "lat",
    LngKey:       "lng",
    LatCenter:    37.7749,
    LngCenter:    -122.4194,
    RadiusMeters: 1000.0, // 1 km radius
    VariantID:    "test_variant",
    ValueData:    "test_value_data",
}
```

Matches if the key `user_location` is within a specified radius from a center point defined by latitude and longitude. The radius is in meters.

#### DateTimeRule

```go
DateTimeRule: &rule.DateTimeRule{
    Key:        "event_time",
    After:     time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC),
    Before:    time.Date(2023, 10, 2, 0, 0, 0, 0, time.UTC),
    VariantID: "test_variant",
    ValueData: "test_value_data",
}
```

Matches if the key `event_time` is within a specified range of time. The `After` and `Before` fields are `time.Time` values.

#### SemVerRule

```go
SemVerRule: &rule.SemVerRule{
    Key:        "app_version",
    Constraint: ">= 2.5.0, < 3.0.0-beta",
    VariantID:  "semver_variant",
    ValueData:  "semver_value_data",
}
```

Matches if the key `app_version` satisfies the specified semantic versioning constraint. The constraint is a string that follows the [SemVer specification](https://semver.org/).

#### CronRule

```go
CronRule: &rule.CronRule{
    Key:        "cron_schedule",
    CronSpec:  "0 9 * * MON-FRI",
    Duration:  8 * time.Hour,
    VariantID: "cron_variant",
    ValueData: "cron_value_data",
}
```

Matches if the key `cron_schedule` (which should be a time.Time value) would be in the range of the specified cron expression + duration. For example, if the cron expression is `0 9 * * MON-FRI`, it will match every weekday at 9 AM, and the duration will extend the match to 8 hours after that time.

### Control Rules

Control rules are used to combine, negate, or override other rules. They can be used to create complex conditions based on multiple rules.

The control rules include:

-   [AndRule](#andrule)
-   [OrRule](#orrule)
-   [NotRule](#notrule)
-   [OverrideRule](#overriderule)

#### AndRule

```go
AndRule: &rule.AndRule{
    Rules: []ConcreteRule{
        {ContainsRule: &ContainsRule{
            Key:       "test_key1",
            Substring: "value1",
        }},
        {ContainsRule: &ContainsRule{
            Key:       "test_key2",
            Substring: "value2",
            VariantID: "variant2",
        }},
        ValueData: "combined_value_data",
    },
}
```

Matches if all the rules in the `Rules` slice match. This is a logical AND operation. An example matching value would be "value1value2" for the keys `test_key1` and `test_key2`.

These do not declare 'VariantID', it is combined logically from the rules in the `Rules` slice. The `ValueData` is used to provide a value when the rule matches. If the rules have their own `ValueData`, it will be ignored in favor of the `ValueData` in the `AndRule`.

#### OrRule

```go
OrRule: &rule.OrRule{
    Rules: []ConcreteRule{
        {ContainsRule: &ContainsRule{
            Key:       "test_key1",
            Substring: "value1",
            VariantID: "variant1",
        }},
        {ContainsRule: &ContainsRule{
            Key:       "test_key2",
            Substring: "value2",
            VariantID: "variant2",
        }},
        ValueData: "or_value_data",
    },
}
```

Matches if any of the rules in the `Rules` slice match. This is a logical OR operation. An example matching value would be "value1" for `test_key1` or "value2" for `test_key2`, or "value1value2" for both keys.

These do not declare 'ValueData', it is combined logically from the rules in the `Rules` slice. The `VariantID` is used to provide a value when the rule matches. If the rules have their own `VariantID`, it will be ignored in favor of the `VariantID` in the `OrRule`.

#### NotRule

```go
NotRule: &rule.NotRule{
    Rule: &rule.ContainsRule{
        Key:       "test_key",
        Substring: "value",
        VariantID: "variant",
    },
    ValueData: "data",
}
```

Matches if the rule does not match. This is a logical NOT operation. For example, if the key `test_key` does not contain the substring `value`, it will match.

These do not declare 'VariantID', it uses the one from the rule in the `Rule` field. The `ValueData` is used to provide a value when the rule matches. If the rule has its own `ValueData`, it will be ignored in favor of the `ValueData` in the `NotRule`.

#### OverrideRule

```go
OverrideRule: &rule.OverrideRule{
    ValueData: "override_value",
    VariantID: "override_variant",
}
```

Matches always, and overrides the value returned by the OpenFeature SDK with the `ValueData` and `VariantID` provided. This is useful for temporary overrides or maintenance windows where you want to force a specific value regardless of other rules.

If there are multiple `OverrideRule`s, the one with the highest priority will be used. If no priority is set, the first one encountered will be used. If another rule has a higher priority, it will override the `OverrideRule`. If you want to ensure that an `OverrideRule` always takes precedence, set its priority to a very high value (e.g., 1000).

### Example

For a complete example, look at [cmd/example/main.go](cmd/example/main.go).

### Editor

Instead of manually creating flags (which can be done with some go code), you can use the editor in this repository to manage flags. To ues it, you can either clone this repo and run

```bash
MONGODB_URI=<your_mongodb_uri> go run cmd/editor/main.go
# or for Testing purposes
USE_TESTCONTAINER=true go run cmd/editor/main.go
```

or you can use the Docker image:

```bash
docker run -p 8080:8080 -e MONGODB_URI=<your_mongodb_uri> zackarysantana/mongo-openfeature-go-editor
```
