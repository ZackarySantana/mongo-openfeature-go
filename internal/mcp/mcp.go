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
		server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
	)

	s.AddTool(se.someTool())
	s.AddResourceTemplate(se.getFeatureFlag())
	s.AddResource(se.getFeatureFlags())

	serve := os.Getenv("MCP_SERVE")

	switch strings.ToLower(serve) {
	case "sse":
		fmt.Println("Starting MCP Server Side Events server on :8080")
		if err := server.NewSSEServer(s).Start("0.0.0.0:8080"); err != nil {
			return fmt.Errorf("running SSE server: %w", err)
		}
	case "http":
		fmt.Println("Starting MCP server on :8080")
		if err := server.NewStreamableHTTPServer(s).Start("0.0.0.0:8080"); err != nil {
			return fmt.Errorf("running MCP server: %w", err)
		}
	default:
		// The only option is stdio
		fmt.Println("Starting MCP stdio server")
		if err := server.ServeStdio(s); err != nil {
			return fmt.Errorf("running stdio server: %w", err)
		}
	}

	return nil
}

func (se *mcpServer) getFeatureFlag() (mcp.ResourceTemplate, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) {
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

func (se *mcpServer) getFeatureFlags() (mcp.Resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) {
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

func (se *mcpServer) someTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return mcp.NewTool("calculate",
			mcp.WithDescription("Perform basic arithmetic operations"),
			mcp.WithString("operation",
				mcp.Required(),
				mcp.Description("The operation to perform (add, subtract, multiply, divide)"),
				mcp.Enum("add", "subtract", "multiply", "divide"),
			),
			mcp.WithNumber("x",
				mcp.Required(),
				mcp.Description("First number"),
			),
			mcp.WithNumber("y",
				mcp.Required(),
				mcp.Description("Second number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			// Using helper functions for type-safe argument access
			op, err := request.RequireString("operation")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			x, err := request.RequireFloat("x")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			y, err := request.RequireFloat("y")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var result float64
			switch op {
			case "add":
				result = x + y
			case "subtract":
				result = x - y
			case "multiply":
				result = x * y
			case "divide":
				if y == 0 {
					return mcp.NewToolResultError("cannot divide by zero"), nil
				}
				result = x / y
			default:
				return mcp.NewToolResultError("unknown operation"), nil
			}

			return mcp.NewToolResultText(fmt.Sprintf("%.2f", result)), nil
		}
}
