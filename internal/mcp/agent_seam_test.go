// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/agent"
)

// testAgent injects downstream MCP customizations through the agent seam.
type testAgent struct {
	agent.Base
	server  agent.MCPServer
	exclude map[string]bool
}

func (a testAgent) MCPServer() agent.MCPServer      { return a.server }
func (a testAgent) IncludeMCPFeature(f string) bool { return !a.exclude[f] }

func installAgent(t *testing.T, a agent.Agent) {
	t.Helper()
	agent.Set(a)
	t.Cleanup(func() { agent.Set(nil) })
}

func TestServerIdentityDefaults(t *testing.T) {
	agent.Set(nil)
	assert.Equal(t, defaultServerName, serverName())
	assert.Equal(t, defaultServerTitle, serverTitle())
	assert.Equal(t, defaultServerInstructions, serverInstructions())
}

func TestAgentOverridesServerIdentity(t *testing.T) {
	installAgent(t, testAgent{server: agent.MCPServer{
		Name:         "bytecloud-supabase-cli",
		Title:        "ByteCloud Supabase CLI",
		Instructions: "Tools for managing ByteCloud Supabase workspaces.",
	}})

	assert.Equal(t, "bytecloud-supabase-cli", serverName())
	assert.Equal(t, "ByteCloud Supabase CLI", serverTitle())
	assert.Equal(t, "Tools for managing ByteCloud Supabase workspaces.", serverInstructions())

	// health_check echoes the overridden identity end to end.
	cs := connect(t, Options{Features: []string{featureDatabase}})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "health_check",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	var out struct {
		Server string `json:"server"`
	}
	require.NoError(t, json.Unmarshal([]byte(resultText(t, res)), &out))
	assert.Equal(t, "bytecloud-supabase-cli", out.Server)
}

func TestAgentPartialIdentityKeepsUpstreamFields(t *testing.T) {
	installAgent(t, testAgent{server: agent.MCPServer{Name: "bytecloud-supabase-cli"}})
	assert.Equal(t, "bytecloud-supabase-cli", serverName())
	assert.Equal(t, defaultServerTitle, serverTitle())
	assert.Equal(t, defaultServerInstructions, serverInstructions())
}

func TestAgentExcludedFeatureIsGone(t *testing.T) {
	installAgent(t, testAgent{exclude: map[string]bool{featurePages: true}})

	// Gone from the default feature set and from health_check's feature list.
	assert.NotContains(t, defaultFeatures(), featurePages)
	assert.Contains(t, defaultFeatures(), featureDatabase)

	// Cannot be resurrected via --features.
	err := validateOptions(Options{Features: []string{featurePages}})
	require.ErrorContains(t, err, "unsupported features: pages")

	// Its tool names no longer validate as --disabled-tools entries.
	err = validateOptions(Options{DisabledTools: []string{"list_pages_projects"}})
	require.ErrorContains(t, err, "unsupported disabled-tools: list_pages_projects")

	// Its tools are not registered under the default feature set.
	cs := connect(t, Options{})
	names := listToolNames(t, cs)
	assert.False(t, names["list_pages_projects"], "excluded pages tools must not register")
	assert.True(t, names["health_check"], "feature-less tools stay")
	assert.True(t, names["execute_sql"], "unrelated feature groups stay")
}

func TestNoAgentKeepsFullFeatureSet(t *testing.T) {
	agent.Set(nil)
	assert.Contains(t, defaultFeatures(), featurePages)
	require.NoError(t, validateOptions(Options{Features: []string{featurePages}}))
}
