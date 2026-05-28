package editor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"github.com/zackarysantana/mongo-openfeature-go/src/rule"
)

const assistantRuleContext = `
Rule types use camelCase keys inside ConcreteRule objects (e.g. exactMatchRule, andRule).
When nesting rules in andRule, orRule, or notRule, only the top-level rule should include ValueData and Priority; nested rules include VariantID only.
Rules are evaluated by priority (higher numbers take precedence).
`

// openRouterTools returns OpenAI-compatible tool definitions for flag management.
func openRouterTools() []map[string]any {
	return []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "get_feature_flag",
				"description": "Retrieve a feature flag by name.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "The name of the feature flag to retrieve.",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "get_all_feature_flags",
				"description": "Retrieve all feature flags.",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "insert_feature_flag",
				"description": "Insert a new feature flag definition.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"flag_name": map[string]any{
							"type":        "string",
							"description": "The unique name of the feature flag.",
						},
						"default_value_json": map[string]any{
							"type":        "string",
							"description": "The default value as a JSON string (e.g. true, \"hello\", 123).",
						},
						"default_variant": map[string]any{
							"type":        "string",
							"description": "The default variant name (e.g. on, off, control).",
						},
						"rules_json": map[string]any{
							"type":        "string",
							"description": "Optional JSON array string of ConcreteRule objects.",
						},
					},
					"required": []string{"flag_name", "default_value_json", "default_variant"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "partial_update_feature_flag",
				"description": "Partially update an existing feature flag. Only provided fields are changed.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"flag_name": map[string]any{
							"type":        "string",
							"description": "The unique name of the feature flag to update.",
						},
						"default_value_json": map[string]any{
							"type":        "string",
							"description": "Optional new default value as a JSON string.",
						},
						"default_variant": map[string]any{
							"type":        "string",
							"description": "Optional new default variant name.",
						},
						"rules_json": map[string]any{
							"type":        "string",
							"description": "Optional JSON array string that replaces all existing rules.",
						},
						"append_rules_json": map[string]any{
							"type":        "string",
							"description": "Optional JSON array string of rules to append.",
						},
					},
					"required": []string{"flag_name"},
				},
			},
		},
	}
}

func executeAssistantTool(ctx context.Context, h *WebHandler, name, argsJSON string) (string, error) {
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	switch name {
	case "get_feature_flag":
		flagName, ok := args["name"].(string)
		if !ok || flagName == "" {
			return "", fmt.Errorf("missing required argument 'name'")
		}
		def, err := h.client.GetFlag(ctx, flagName)
		if err != nil {
			return "", fmt.Errorf("getting feature flag %q: %w", flagName, err)
		}
		return marshalToolResult(def)

	case "get_all_feature_flags":
		flags, err := h.client.GetAllFlags(ctx)
		if err != nil {
			return "", fmt.Errorf("getting all feature flags: %w", err)
		}
		return marshalToolResult(flags)

	case "insert_feature_flag":
		flagName, _ := args["flag_name"].(string)
		defaultValueJSON, _ := args["default_value_json"].(string)
		defaultVariant, _ := args["default_variant"].(string)
		if flagName == "" || defaultValueJSON == "" || defaultVariant == "" {
			return "", fmt.Errorf("flag_name, default_value_json, and default_variant are required")
		}

		var defaultValue any
		if err := json.Unmarshal([]byte(defaultValueJSON), &defaultValue); err != nil {
			return "", fmt.Errorf("invalid default_value_json: %w", err)
		}

		flagDef := flag.Definition{
			FlagName:       flagName,
			DefaultValue:   defaultValue,
			DefaultVariant: defaultVariant,
		}

		if rulesJSON, _ := args["rules_json"].(string); rulesJSON != "" {
			var rules []rule.ConcreteRule
			if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
				return "", fmt.Errorf("invalid rules_json: %w", err)
			}
			flagDef.Rules = rules
		}

		exists, err := h.client.FlagExists(ctx, flagName)
		if err != nil {
			return "", fmt.Errorf("checking flag existence: %w", err)
		}
		if exists {
			return "", fmt.Errorf("feature flag %q already exists", flagName)
		}
		if err := h.client.SetFlag(ctx, flagDef); err != nil {
			return "", fmt.Errorf("inserting feature flag: %w", err)
		}
		return fmt.Sprintf("Successfully inserted feature flag %q.", flagName), nil

	case "partial_update_feature_flag":
		flagName, _ := args["flag_name"].(string)
		if flagName == "" {
			return "", fmt.Errorf("missing required argument 'flag_name'")
		}

		exists, err := h.client.FlagExists(ctx, flagName)
		if err != nil {
			return "", fmt.Errorf("checking flag existence: %w", err)
		}
		if !exists {
			return "", fmt.Errorf("feature flag %q does not exist", flagName)
		}

		updates := make(map[string]any)

		if defaultValueJSON, ok := args["default_value_json"].(string); ok && defaultValueJSON != "" {
			var defaultValue any
			if err := json.Unmarshal([]byte(defaultValueJSON), &defaultValue); err != nil {
				return "", fmt.Errorf("invalid default_value_json: %w", err)
			}
			updates["defaultValue"] = defaultValue
		}
		if defaultVariant, ok := args["default_variant"].(string); ok && defaultVariant != "" {
			updates["defaultVariant"] = defaultVariant
		}
		if rulesJSON, ok := args["rules_json"].(string); ok && rulesJSON != "" {
			var rules []rule.ConcreteRule
			if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
				return "", fmt.Errorf("invalid rules_json: %w", err)
			}
			updates["rules"] = rules
		}
		if appendRulesJSON, ok := args["append_rules_json"].(string); ok && appendRulesJSON != "" {
			var appendRules []rule.ConcreteRule
			if err := json.Unmarshal([]byte(appendRulesJSON), &appendRules); err != nil {
				return "", fmt.Errorf("invalid append_rules_json: %w", err)
			}
			updates["append_rules"] = appendRules
		}
		if len(updates) == 0 {
			return fmt.Sprintf("No update fields provided for flag %q.", flagName), nil
		}
		if err := h.client.PartialUpdateFlag(ctx, flagName, updates); err != nil {
			return "", fmt.Errorf("updating feature flag: %w", err)
		}
		return fmt.Sprintf("Successfully updated feature flag %q.", flagName), nil

	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func marshalToolResult(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func assistantSystemPrompt(currentFlag string) string {
	prompt := "You are a helpful assistant for managing OpenFeature feature flags stored in MongoDB. " +
		"Use the provided tools to list, retrieve, create, update, and analyze flags. " +
		"When analyzing flags, explain targeting rules clearly and note potential issues such as conflicting priorities or missing defaults. " +
		"When creating or updating flags, use valid ConcreteRule JSON and confirm what changed.\n\n" +
		assistantRuleContext
	if currentFlag != "" {
		prompt += fmt.Sprintf("\n\nThe user is currently viewing the flag %q in the editor.", currentFlag)
	}
	return prompt
}
