// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connect wires a client to a newly created server via an in-memory transport and returns the active client session.
func connect(t *testing.T, opts Options) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()

	srv := newServer(opts, io.Discard)
	ss, err := srv.Connect(ctx, serverT, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func listToolNames(t *testing.T, cs *mcp.ClientSession) map[string]bool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := make(map[string]bool, len(res.Tools))
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	return names
}

// resultText returns the concatenated text content from a tool result.
func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var out string
	for _, c := range res.Content {
		tc, ok := c.(*mcp.TextContent)
		require.True(t, ok, "expected text content")
		out += tc.Text
	}
	return out
}

func TestHealthCheckRoundTrip(t *testing.T) {
	cs := connect(t, Options{ReadOnly: true, Features: []string{featureDatabase}})

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "health_check",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	var out struct {
		Server   string   `json:"server"`
		ReadOnly bool     `json:"read_only"`
		Features []string `json:"features"`
	}
	require.NoError(t, json.Unmarshal([]byte(resultText(t, res)), &out))
	assert.Equal(t, serverName(), out.Server)
	assert.True(t, out.ReadOnly)
	assert.Equal(t, []string{featureDatabase}, out.Features)
}

func TestHealthCheckAlwaysExposed(t *testing.T) {
	// health_check has no feature group, so it is exposed regardless of which features are enabled.
	cs := connect(t, Options{Features: []string{featureStorage}})
	assert.True(t, listToolNames(t, cs)["health_check"])
}

func TestDisabledToolsHidesHealthCheck(t *testing.T) {
	// The denylist is the final filter; even unconditional tools can be removed by it.
	cs := connect(t, Options{DisabledTools: []string{"health_check"}})
	assert.False(t, listToolNames(t, cs)["health_check"])
}

func TestExecuteSQLGatedByDatabaseFeature(t *testing.T) {
	// execute_sql belongs to the database group.
	enabled := connect(t, Options{Features: []string{featureDatabase}})
	assert.True(t, listToolNames(t, enabled)["execute_sql"])

	disabled := connect(t, Options{Features: []string{featureFunctions}})
	assert.False(t, listToolNames(t, disabled)["execute_sql"])
}

func TestExecuteSQLHiddenInReadOnly(t *testing.T) {
	// execute_sql runs arbitrary SQL and is treated as a write tool: hidden in read-only mode — use the built-in read-only tools instead.
	cs := connect(t, Options{ReadOnly: true, Features: []string{featureDatabase}})
	assert.False(t, listToolNames(t, cs)["execute_sql"])
}

func TestExecuteSQLRequiresSQL(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureDatabase}})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "execute_sql",
		Arguments: map[string]any{"workspace_id": "ws-test", "sql": "   "},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "sql is required")
}
