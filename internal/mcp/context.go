package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func newToolResultResponseWithContext(name, uri string, data any) *mcp.CallToolResult {
	response := map[string]any{
		name:      data,
		"context": contextGet,
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling response to JSON: %v", err))
	}
	return mcp.NewToolResultResource(name, mcp.TextResourceContents{
		URI:      uri,
		MIMEType: "application/json",
		Text:     string(responseJSON),
	})
}

const contextGet = `
{
 "ExactMatchRule": "Matches when a context key exactly equals a specified value",
 "RegexRule": "Matches when a context key matches a regular expression pattern",
 "ExistsRule": "Matches when a specified key exists in the evaluation context",
 "FractionalRule": "Matches a percentage of users based on a hash of the key and its value",
 "RangeRule": "Matches when a numeric context key falls within a specified min/max range",
 "InListRule": "Matches when a context key's value is contained in a predefined list of values",
 "PrefixRule": "Matches when a context key's string value starts with a specified prefix",
 "SuffixRule": "Matches when a context key's string value ends with a specified suffix",
 "ContainsRule": "Matches when a context key's string value contains a specified substring",
 "IPRangeRule": "Matches when an IP address context key falls within specified CIDR ranges",
 "GeoFenceRule": "Matches when latitude/longitude coordinates are within a specified radius from a center point",
 "DateTimeRule": "Matches when a time context key falls within a specified time range",
 "SemVerRule": "Matches when a semantic version context key satisfies a version constraint expression",
 "CronRule": "Matches when a time context key falls within a cron schedule plus duration window",
 "AndRule": "Matches only when all nested rules match (logical AND operation)",
 "OrRule": "Matches when any nested rule matches (logical OR operation)",
 "NotRule": "Matches when the nested rule does not match (logical NOT operation)",
 "OverrideRule": "Always matches and returns a specific value, can be overridden by rules with higher priority"
}`

const contextPost = `
{
  "rule_types": {
    "exactMatchRule": {
      "description": "Matches when a context key exactly equals a specified value.",
      "fields": {
        "Key": "string - context key to check",
        "KeyValue": "string - value to match exactly",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority (higher = takes precedence)",
        "ValueData": "any - value to return when matched"
      }
    },
    "regexRule": {
      "description": "Matches when a context key matches a regular expression pattern.",
      "fields": {
        "Key": "string - context key to check",
        "Pattern": "string - regex pattern to match",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "existsRule": {
      "description": "Matches when a specified key exists in the evaluation context.",
      "fields": {
        "Key": "string - context key to check for existence",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "fractionalRule": {
      "description": "Matches a percentage of users based on a hash of the key and its value.",
      "fields": {
        "Key": "string - context key to hash",
        "Percentage": "float64 - percentage (0.0-100.0) of users to match",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "rangeRule": {
      "description": "Matches when a numeric context key falls within a specified min/max range.",
      "fields": {
        "Key": "string - context key to check",
        "Min": "float64 - minimum value",
        "Max": "float64 - maximum value",
        "ExclusiveMin": "bool - whether min is exclusive (default: false)",
        "ExclusiveMax": "bool - whether max is exclusive (default: false)",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "inListRule": {
      "description": "Matches when a context key's value is contained in a predefined list of values.",
      "fields": {
        "Key": "string - context key to check",
        "Items": "array - list of values to match against",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "prefixRule": {
      "description": "Matches when a context key's string value starts with a specified prefix.",
      "fields": {
        "Key": "string - context key to check",
        "Prefix": "string - prefix to match",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "suffixRule": {
      "description": "Matches when a context key's string value ends with a specified suffix.",
      "fields": {
        "Key": "string - context key to check",
        "Suffix": "string - suffix to match",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "containsRule": {
      "description": "Matches when a context key's string value contains a specified substring.",
      "fields": {
        "Key": "string - context key to check",
        "Substring": "string - substring to match",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "ipRangeRule": {
      "description": "Matches when an IP address context key falls within specified CIDR ranges.",
      "fields": {
        "Key": "string - context key containing IP address",
        "CIDRs": "array of strings - list of CIDR ranges to match",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "geoFenceRule": {
      "description": "Matches when latitude/longitude coordinates are within a specified radius from a center point.",
      "fields": {
        "LatKey": "string - context key for latitude",
        "LngKey": "string - context key for longitude",
        "LatCenter": "float64 - center latitude",
        "LngCenter": "float64 - center longitude",
        "RadiusMeters": "float64 - radius in meters",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "dateTimeRule": {
      "description": "Matches when a time context key falls within a specified time range.",
      "fields": {
        "Key": "string - context key containing time.Time",
        "After": "string (RFC3339) - start of time range",
        "Before": "string (RFC3339) - end of time range",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "semVerRule": {
      "description": "Matches when a semantic version context key satisfies a version constraint expression.",
      "fields": {
        "Key": "string - context key containing version string",
        "Constraint": "string - semver constraint (e.g., '>= 1.2.3, < 2.0.0')",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "cronRule": {
      "description": "Matches when a time context key falls within a cron schedule plus duration window.",
      "fields": {
        "Key": "string - context key containing time.Time (empty string uses time.Now())",
        "CronSpec": "string - cron expression (e.g., '0 9 * * MON-FRI')",
        "Duration": "int64 - window duration from cron trigger, in nanoseconds (Go time.Duration)",
        "VariantID": "string - variant identifier",
        "Priority": "int - rule priority",
        "ValueData": "any - value to return when matched"
      }
    },
    "andRule": {
      "description": "Matches only when all nested rules match (logical AND operation). Only the top-level andRule should have ValueData and Priority; nested rules must not include ValueData or Priority, but do include VariantID.",
      "fields": {
        "Rules": "array of ConcreteRule - list of rules that must all match",
        "Priority": "int - rule priority (top-level only)",
        "ValueData": "any - value to return when matched (top-level only)"
      }
    },
    "orRule": {
      "description": "Matches when any nested rule matches (logical OR operation). Only the top-level orRule should have ValueData and Priority; nested rules must not include ValueData or Priority, but do include VariantID.",
      "fields": {
        "Rules": "array of ConcreteRule - list of rules where any can match",
        "Priority": "int - rule priority (top-level only)",
        "ValueData": "any - value to return when matched (top-level only)"
      }
    },
    "notRule": {
      "description": "Matches when the nested rule does not match (logical NOT operation). Only the top-level notRule should have ValueData and Priority; the nested rule must not include ValueData or Priority, but does include VariantID.",
      "fields": {
        "Rule": "ConcreteRule - rule to negate",
        "Priority": "int - rule priority (top-level only)",
        "ValueData": "any - value to return when matched (top-level only)"
      }
    },
    "overrideRule": {
      "description": "Always matches and returns a specific value, can be overridden by rules with higher priority.",
      "fields": {
        "ValueData": "any - value to return",
        "Priority": "int - rule priority",
        "VariantID": "string - variant identifier"
      }
    }
  },
  "concrete_rule": {
    "description": "A ConcreteRule is an object with a single key, where the key is the rule type in camelCase (e.g., 'exactMatchRule', 'regexRule', 'andRule', etc.), and the value is the rule object itself. When nesting rules (e.g., in 'andRule', 'orRule', 'notRule'), only the top-level rule should have 'ValueData' and 'Priority'; nested rules must not include 'ValueData' or 'Priority', but do include 'VariantID'.",
    "example": {
      "exactMatchRule": {
        "Key": "user_id",
        "KeyValue": "123",
        "VariantID": "beta",
        "Priority": 10,
        "ValueData": true
      }
    },
    "nested_example": {
      "andRule": {
        "Rules": [
          {
            "exactMatchRule": {
              "Key": "user_id",
              "KeyValue": "123",
              "VariantID": "beta"
            }
          },
          {
            "regexRule": {
              "Key": "email",
              "Pattern": ".*@example.com",
              "VariantID": "email"
            }
          }
        ],
        "Priority": 100,
        "ValueData": true
      }
    }
  },
  "flag_definition": {
    "description": "A feature flag definition with rules for evaluation.",
    "fields": {
      "FlagName": "string - unique name of the feature flag",
      "DefaultValue": "any - default value when no rules match",
      "DefaultVariant": "string - default variant identifier",
      "Rules": "array of ConcreteRule objects - list of rules to evaluate in priority order"
    },
    "example": {
      "FlagName": "my_flag",
      "DefaultValue": false,
      "DefaultVariant": "off",
      "Rules": [
        {
          "exactMatchRule": {
            "Key": "user_id",
            "KeyValue": "123",
            "VariantID": "on",
            "Priority": 100,
            "ValueData": true
          }
        },
        {
          "andRule": {
            "Rules": [
              {
                "existsRule": {
                  "Key": "email",
                  "VariantID": "has_email"
                }
              },
              {
                "regexRule": {
                  "Key": "email",
                  "Pattern": ".*@example.com",
                  "VariantID": "email"
                }
              }
            ],
            "Priority": 50,
            "ValueData": true
          }
        }
      ]
    }
  },
  "rule_evaluation": {
    "description": "Rules are evaluated by priority (higher numbers take precedence). When multiple rules have the same priority, override rules take precedence over non-override rules. The first matching rule at the highest priority level is used."
  }
}
`
