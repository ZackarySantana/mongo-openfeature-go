package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Some applications don't work well with resources and dynamic resources, so we provide them as tools as well

func (se *mcpServer) getFeatureFlagTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return mcp.NewTool("get_feature_flag",
			mcp.WithDescription("Retrieve a feature flag by name"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("The name of the feature flag to retrieve"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := request.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			featureFlag, err := se.ofClient.GetFlag(ctx, name)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting feature flag '%s': %v", name, err)), nil
			}
			return newToolResultResponseWithContext(name, fmt.Sprintf("feature_flags://%s", name), featureFlag), nil
		}
}

func (se *mcpServer) getFeatureFlagsTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return mcp.NewTool("get_all_feature_flags",
			mcp.WithDescription("Retrieve all feature flags"),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			featureFlags, err := se.ofClient.GetAllFlags(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("getting all feature flags: %v", err)), nil
			}
			return newToolResultResponseWithContext("all_feature_flags", "feature_flags://all", featureFlags), nil
		}
}
