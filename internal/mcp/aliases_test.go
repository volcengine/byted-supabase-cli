// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callByName invokes a tool by name with empty arguments and returns the result/error
// as the client sees them. A nil error with res.IsError means the tool's handler ran
// (and returned a business validation error); a non-nil error means the server rejected
// the call at the protocol layer (e.g. "unknown tool").
func callByName(t *testing.T, cs *mcp.ClientSession, name string) (*mcp.CallToolResult, error) {
	t.Helper()
	return cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
}

// TestAliasRoutesToCanonical verifies that calling a tool by its backward-compatible old
// name reaches the renamed canonical tool: the call is validated against the canonical
// tool's schema (which requires workspace_id) and reported as a tool error — not rejected
// as an unknown tool at the protocol layer, which is what a missing route would produce.
func TestAliasRoutesToCanonical(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureAccount}})
	for _, alias := range []string{
		"restore_workspace", // -> start_workspace
		"pause_workspace",   // -> stop_workspace
	} {
		res, err := callByName(t, cs, alias)
		require.NoError(t, err, "alias %q must route, not fail as an unknown tool", alias)
		assert.True(t, res.IsError, "alias %q should hit the canonical tool's required-arg validation", alias)
		assert.Contains(t, resultText(t, res), "workspace_id")
	}
}

// TestComputeAliasRoutesToCanonical verifies the hidden update_compute_settings alias
// reaches modify_compute_settings (a no-op request is rejected by the handler), and that
// the alias is not advertised in tools/list.
func TestComputeAliasRoutesToCanonical(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureCompute}})
	assert.True(t, listToolNames(t, cs)["modify_compute_settings"])
	assert.False(t, listToolNames(t, cs)["update_compute_settings"], "the update alias must stay hidden")

	res, err := callByName(t, cs, "update_compute_settings")
	require.NoError(t, err, "update_compute_settings must route to modify_compute_settings")
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "nothing to modify")
}

// TestAliasUnavailableInReadOnly verifies that an alias of a mutating tool is not callable
// in read-only mode (the canonical tool is hidden, so no alias route is installed).
func TestAliasUnavailableInReadOnly(t *testing.T) {
	cs := connect(t, Options{ReadOnly: true, Features: []string{featureAccount}})
	_, err := callByName(t, cs, "restore_workspace")
	require.Error(t, err, "alias of a hidden mutating tool must not be callable")
	assert.Contains(t, err.Error(), "unknown tool")
}

// TestAliasDisabledByOldName verifies that denylisting a tool by its old name removes only
// the alias route: the canonical tool stays registered and callable, while the old name
// no longer resolves.
func TestAliasDisabledByOldName(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureAccount}, DisabledTools: []string{"restore_workspace"}})
	names := listToolNames(t, cs)
	assert.True(t, names["start_workspace"], "canonical tool stays available when only its alias is disabled")

	// The disabled alias no longer routes.
	_, err := callByName(t, cs, "restore_workspace")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")

	// The canonical name still reaches the tool (validated against its required-arg schema).
	res, err := callByName(t, cs, "start_workspace")
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "workspace_id")
}

// TestDisabledCanonicalRemovesAlias verifies that denylisting the canonical name drops the
// whole tool, including its alias route.
func TestDisabledCanonicalRemovesAlias(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureAccount}, DisabledTools: []string{"start_workspace"}})
	assert.False(t, listToolNames(t, cs)["start_workspace"])

	for _, name := range []string{"start_workspace", "restore_workspace"} {
		_, err := callByName(t, cs, name)
		require.Error(t, err, "%s must be gone when the canonical tool is disabled", name)
		assert.Contains(t, err.Error(), "unknown tool")
	}
}

// TestValidateOptionsAcceptsAliasInDisabledTools verifies that a tool's old (alias) name is
// a valid --disabled-tools entry, so existing configs referencing pre-rename names still pass
// startup validation.
func TestValidateOptionsAcceptsAliasInDisabledTools(t *testing.T) {
	require.NoError(t, validateOptions(Options{
		DisabledTools: []string{"restore_workspace", "pause_workspace", "update_compute_settings"},
	}))
}
