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

			// Test if the flag already exists
			exists, err := se.ofClient.FlagExists(ctx, flagName)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("error checking existence of flag '%s': %v", flagName, err)), nil
			}
			if exists {
				return mcp.NewToolResultError(fmt.Sprintf("feature flag '%s' already exists", flagName)), nil
			}

			// Insert the flag definition using the client
			if err := se.ofClient.SetFlag(ctx, flagDef); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to insert feature flag '%s': %v", flagName, err)), nil
			}

			// Return success
			return mcp.NewToolResultText(fmt.Sprintf("Successfully inserted feature flag '%s'.", flagName)), nil
		}
}

func (se *mcpServer) partialUpdateFeatureFlagTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return mcp.NewTool("partial_update_feature_flag",
			mcp.WithDescription("Partially updates an existing feature flag definition. Only the provided fields will be changed."),
			mcp.WithDescription(
				"This tool updates specific fields of an existing feature flag. Provide the flag name and any fields you wish to change.\n\nContext:\n"+contextPost,
			),
			mcp.WithString("flag_name",
				mcp.Required(),
				mcp.Description("The unique name of the feature flag to update."),
			),
			mcp.WithString("default_value_json",
				mcp.Description("An optional new default value for the flag, as a JSON string (e.g., 'true', '\"hello\"', '123')."),
			),
			mcp.WithString("default_variant",
				mcp.Description("An optional new default variant name for the flag (e.g., 'on', 'off', 'control')."),
			),
			mcp.WithString("rules_json",
				mcp.Description("An optional new JSON array string for the targeting rules. This will completely replace existing rules."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract the required flag name
			flagName, err := request.RequireString("flag_name")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("missing required argument 'flag_name': %v", err)), nil
			}

			// First, check if the flag exists. If not, we can't update it.
			exists, err := se.ofClient.FlagExists(ctx, flagName)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("error checking existence of flag '%s': %v", flagName, err)), nil
			}
			if !exists {
				return mcp.NewToolResultError(fmt.Sprintf("feature flag '%s' does not exist and cannot be updated", flagName)), nil
			}

			// Build a map of fields to update. The keys must match the BSON tags in the flag.Definition struct.
			updates := make(map[string]any)

			if defaultValueJSON := request.GetString("default_value_json", ""); defaultValueJSON != "" {
				var defaultValue any
				if err := json.Unmarshal([]byte(defaultValueJSON), &defaultValue); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid JSON for 'default_value_json': %v", err)), nil
				}
				updates["defaultValue"] = defaultValue
			}

			if defaultVariant := request.GetString("default_variant", ""); defaultVariant != "" {
				updates["defaultVariant"] = defaultVariant
			}

			if rulesJSON := request.GetString("rules_json", ""); rulesJSON != "" {
				var rules []rule.ConcreteRule
				if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("invalid JSON for 'rules_json': %v", err)), nil
				}
				updates["rules"] = rules
			}

			// If no update fields were provided, there's nothing to do.
			if len(updates) == 0 {
				return mcp.NewToolResultText(fmt.Sprintf("No update fields provided for flag '%s'. Nothing was changed.", flagName)), nil
			}

			// Perform the partial update using the client
			if err := se.ofClient.PartialUpdateFlag(ctx, flagName, updates); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to update feature flag '%s': %v", flagName, err)), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("Successfully updated feature flag '%s'.", flagName)), nil
		}
}
