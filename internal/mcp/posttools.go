package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

func (se *mcpServer) insertFeatureFlagTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return mcp.NewTool("insert_feature_flag",
			mcp.WithDescription("Insert a new feature flag definition into the system."),
			mcp.WithDescription(
				"This tool inserts a new feature flag definition. Provide the flag name, default value, default variant, and optional rules as described above.\n\nContext:\n"+contextPost,
			),
			mcp.WithString("flag_name",
				mcp.Required(),
				mcp.Description("The unique name of the feature flag."),
			),
			mcp.WithString("default_value_json",
				mcp.Required(),
				mcp.Description("The default value of the flag, as a JSON string (e.g., 'true', '\"hello\"', '123')."),
			),
			mcp.WithString("default_variant",
				mcp.Required(),
				mcp.Description("The default variant name for the flag (e.g., 'on', 'off', 'control')."),
			),
			mcp.WithString("rules_json",
				mcp.Description("An optional JSON array string representing the targeting rules for the flag. Each rule should be a ConcreteRule object."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract required arguments
			flagName, err := request.RequireString("flag_name")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'flag_name': %v", err)), nil
			}

			defaultValueJSON, err := request.RequireString("default_value_json")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'default_value_json': %v", err)), nil
			}

			defaultVariant, err := request.RequireString("default_variant")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'default_variant': %v", err)), nil
			}

			// Parse the default value from JSON
			var defaultValue any
			if err := json.Unmarshal([]byte(defaultValueJSON), &defaultValue); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid JSON format for 'default_value_json': %v", err)), nil
			}

			// Create the flag definition
			flagDef := flag.Definition{
				FlagName:       flagName,
				DefaultValue:   defaultValue,
				DefaultVariant: defaultVariant,
			}

			// Extract optional rules
			rulesJSON := request.GetString("rules_json", "")
			if rulesJSON != "" {
				var rules []rule.ConcreteRule
				if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid JSON format for 'rules_json': %v", err)), nil
				}
				flagDef.Rules = rules
			}

			// Insert the flag definition using the client
			if err := se.ofClient.SetFlag(ctx, flagDef); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to insert feature flag '%s': %v", flagName, err)), nil
			}

			// Return success
			return mcp.NewToolResultText(fmt.Sprintf("Successfully inserted feature flag '%s'.", flagName)), nil
		}
}
