package solo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

type Options struct {
	Debug    bool
	ReadOnly bool
}

type emptyInput struct{}

type result struct {
	Value any `json:"value"`
}

func Serve(ctx context.Context, options Options) error {
	server := NewServer(options, os.Stderr)
	err := server.Run(ctx, &mcp.StdioTransport{})
	if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func NewServer(options Options, logw io.Writer) *mcp.Server {
	level := slog.LevelInfo
	if options.Debug {
		level = slog.LevelDebug
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "byted-supabase-cli-solo",
		Title:   "Byted Supabase CLI (linked branch)",
		Version: "dev",
	}, &mcp.ServerOptions{
		Logger:       slog.New(slog.NewTextHandler(logw, &slog.HandlerOptions{Level: level})),
		Instructions: "Tools for operating the Supabase branch this project is linked to.",
	})
	registerTools(server, options)
	return server
}

func registerTools(server *mcp.Server, options Options) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "solo_health_check",
		Title:       "Linked branch health check",
		Description: "Check that this MCP server is connected and a linked branch is configured.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		profile, err := soloconfig.LoadCurrentProfile()
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(map[string]any{"mode": "solo", "profile": profile.Name, "url": profile.SupabaseURL})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "solo_endpoints_list",
		Title:       "List branch endpoints",
		Description: "List the configured Supabase Auth, REST, Storage, and Functions endpoints.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		client, err := soloconfig.LoadEndpointsClient()
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(client.List())
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "solo_auth_config_get",
		Title:       "Get branch Auth config",
		Description: "Read Supabase Auth configuration using the configured Service Role Key.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		client, err := soloconfig.LoadCurrentProfile()
		if err != nil {
			return nil, nil, err
		}
		authClient, err := soloconfig.NewAuthClient(client)
		if err != nil {
			return nil, nil, err
		}
		value, err := authClient.GetConfig(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(value)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "solo_storage_buckets_list",
		Title:       "List branch Storage buckets",
		Description: "List Storage buckets in the configured Supabase branch.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		client, err := soloconfig.LoadStorageAPI()
		if err != nil {
			return nil, nil, err
		}
		value, err := client.ListBuckets(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(value)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "solo_functions_list",
		Title:       "List branch Functions",
		Description: "List Edge Functions in the configured Supabase branch.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		client, err := soloconfig.LoadFunctionsClient()
		if err != nil {
			return nil, nil, err
		}
		value, err := client.List(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(value)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "solo_secrets_list",
		Title:       "List branch secrets",
		Description: "List secret digests in the configured Supabase branch. Secret values are never returned.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
		client, err := soloconfig.LoadSecretsClient()
		if err != nil {
			return nil, nil, err
		}
		value, err := client.List(ctx)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(value)
	})

	if !options.ReadOnly {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "solo_endpoints_check",
			Title:       "Check branch endpoints",
			Description: "Check the configured Supabase service endpoints.",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
			client, err := soloconfig.LoadEndpointsClient()
			if err != nil {
				return nil, nil, err
			}
			return jsonResult(client.Check(ctx))
		})
	}
}

func jsonResult(value any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, result{Value: value}, nil
}
