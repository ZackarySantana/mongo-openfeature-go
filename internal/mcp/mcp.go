package mcp

import (
	"context"
	"fmt"

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
		// server.WithToolCapabilities(false),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
	)

	s.AddTool(se.someTool())

	fmt.Println("Starting MCP server on :8080")
	if err := server.NewStreamableHTTPServer(s).Start("0.0.0.0:8080"); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	return nil
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
