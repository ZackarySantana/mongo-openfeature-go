package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type mcpServer struct {
	collection *mongo.Collection
	ofClient   *client.Client
	documentID string
}

func Serve(collection *mongo.Collection, ofClient *client.Client, documentID string) error {
	server := &mcpServer{
		collection: collection,
		ofClient:   ofClient,
		documentID: documentID,
	}
	return server.Serve()
}

func (se *mcpServer) Serve() error {
	s := server.NewMCPServer(
		"MongoDB OpenFeature Provider",
		"1.0.0",
		server.WithRecovery(),
		server.WithResourceCapabilities(false, true),
	)

	// Resources
	s.AddResourceTemplate(se.getFeatureFlagResource())
	s.AddResource(se.getFeatureFlagsResource())

	// Tools
	s.AddTool(se.getFeatureFlagTool())
	s.AddTool(se.getFeatureFlagsTool())
	s.AddTool(se.insertFeatureFlagTool())
	s.AddTool(se.partialUpdateFeatureFlagTool())

	serve := os.Getenv("MCP_SERVE")

	switch strings.ToLower(serve) {
	case "sse":
		port := ":8080"
		if envPort := os.Getenv("MCP_PORT"); envPort != "" {
			port = ":" + envPort
		}
		fmt.Println("Starting MCP Server Side Events server on http://localhost" + port)
		if err := server.NewSSEServer(s).Start("0.0.0.0" + port); err != nil {
			return fmt.Errorf("running SSE server: %w", err)
		}
	case "http":
		port := ":8080"
		if envPort := os.Getenv("MCP_PORT"); envPort != "" {
			port = ":" + envPort
		}
		fmt.Println("Starting MCP server on http://localhost" + port)
		if err := server.NewStreamableHTTPServer(s).Start("0.0.0.0" + port); err != nil {
			return fmt.Errorf("running MCP server: %w", err)
		}
	default:
		// The only option is stdio
		// Stdio should not output anything other than the MCP protocol
		if err := server.ServeStdio(s); err != nil {
			return fmt.Errorf("running stdio server: %w", err)
		}
	}

	return nil
}

func (se *mcpServer) getFeatureFlagResource() (mcp.ResourceTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) {
	return mcp.NewResourceTemplate("feature_flags://{name}", "Get Feature Flag",
			mcp.WithTemplateDescription("Returns feature flag details by name"),
			mcp.WithTemplateMIMEType("application/json"),
		), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			name, found := strings.CutPrefix(request.Params.URI, "feature_flags://")
			if !found || name == "" {
				return nil, fmt.Errorf("missing 'name' parameter")
			}

			featureFlag, err := se.ofClient.GetFlag(ctx, name)
			if err != nil {
				return nil, fmt.Errorf("getting feature flag '%s': %w", name, err)
			}
			featureFlagJSON, err := json.Marshal(featureFlag)
			if err != nil {
				return nil, fmt.Errorf("marshaling feature flag '%s' to JSON: %w", name, err)
			}

			response := mcp.TextResourceContents{
				URI:      fmt.Sprintf("feature_flags://%s", name),
				MIMEType: "application/json",
				Text:     string(featureFlagJSON),
			}

			return []mcp.ResourceContents{response}, nil
		}
}

func (se *mcpServer) getFeatureFlagsResource() (mcp.Resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) {
	return mcp.NewResource("feature_flags://all", "Get All Feature Flags",
			mcp.WithResourceDescription("Returns all feature flags"),
			mcp.WithMIMEType("application/json"),
		), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			featureFlags, err := se.ofClient.GetAllFlags(ctx)
			if err != nil {
				return nil, fmt.Errorf("getting all feature flags: %w", err)
			}

			response := []mcp.ResourceContents{}
			for name, ff := range featureFlags {
				ffJSON, err := json.Marshal(ff)
				if err != nil {
					return nil, fmt.Errorf("marshaling feature flag '%s' to JSON: %w", name, err)
				}

				response = append(response, mcp.TextResourceContents{
					URI:      fmt.Sprintf("feature_flags://%s", name),
					MIMEType: "application/json",
					Text:     string(ffJSON),
				})
			}

			return response, nil
		}
}
