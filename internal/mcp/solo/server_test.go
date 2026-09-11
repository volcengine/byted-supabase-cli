package solo

import (
	"context"
	"io"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func connect(t *testing.T, options Options) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := NewServer(options, io.Discard)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "solo-test", Version: "dev"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func TestSoloMCPToolCatalogIsDataPlaneOnly(t *testing.T) {
	session := connect(t, Options{})
	response, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := make(map[string]bool, len(response.Tools))
	for _, tool := range response.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{
		"solo_health_check",
		"solo_endpoints_list",
		"solo_endpoints_check",
		"solo_auth_config_get",
		"solo_storage_buckets_list",
		"solo_functions_list",
		"solo_secrets_list",
	} {
		require.True(t, names[name], "missing Solo MCP tool %s", name)
	}
	for _, name := range []string{
		"list_workspaces",
		"list_branches",
		"restart_branch_compute",
		"list_storage_buckets",
	} {
		require.False(t, names[name], "management MCP tool leaked into Solo server: %s", name)
	}
}

func TestSoloMCPReadOnlyHidesEndpointCheck(t *testing.T) {
	session := connect(t, Options{ReadOnly: true})
	response, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tool := range response.Tools {
		require.NotEqual(t, "solo_endpoints_check", tool.Name)
	}
}

func TestSoloMCPHealthCheckRequiresConfiguredProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	session := connect(t, Options{})
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "solo_health_check",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}
