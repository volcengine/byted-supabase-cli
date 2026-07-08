// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToJSONRoundTrip(t *testing.T) {
	out, err := toJSON(map[string]any{"b": 2, "a": "x"})
	require.NoError(t, err)
	// Indented output for human readability: must contain newlines and two-space indentation.
	assert.Contains(t, out, "\n")
	assert.Contains(t, out, "  \"a\"")
	// Still valid, round-trippable JSON.
	var back map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &back))
	assert.Equal(t, "x", back["a"])
}

func TestToJSONReportsMarshalError(t *testing.T) {
	// A channel cannot be serialised; toJSON must wrap and return the error rather than panic.
	_, err := toJSON(map[string]any{"bad": make(chan int)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to encode result")
}

func TestTextResult(t *testing.T) {
	res := textResult("hello")
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected a TextContent")
	assert.Equal(t, "hello", tc.Text)
}

// TestCatalogInvariants guards the structural invariants of the full tool catalog:
// names are non-empty and unique; feature is either empty (transport-layer tool) or a
// supported official group; and every alias is space-free and globally unique across both
// canonical names and other aliases (so the alias-rewrite map can never be ambiguous).
// A newly added tool that violates these is caught here.
func TestCatalogInvariants(t *testing.T) {
	specs := allSpecs()
	require.NotEmpty(t, specs)

	seen := make(map[string]bool, len(specs))
	for _, s := range specs {
		name := s.meta.name
		assert.NotEmpty(t, name, "every tool must have a name")
		assert.False(t, seen[name], "duplicate tool name %q", name)
		seen[name] = true

		if s.meta.feature != "" {
			assert.True(t, officialFeatures[s.meta.feature],
				"tool %q has unknown feature %q", name, s.meta.feature)
		}
		assert.False(t, strings.Contains(name, " "), "tool name %q must not contain spaces", name)
	}

	// Aliases share the same namespace as canonical names and must be unique within it,
	// otherwise the alias-rewrite middleware would have an ambiguous mapping.
	for _, s := range specs {
		for _, a := range s.meta.aliases {
			assert.NotEmpty(t, a, "tool %q has an empty alias", s.meta.name)
			assert.False(t, strings.Contains(a, " "), "alias %q must not contain spaces", a)
			assert.False(t, seen[a], "alias %q collides with an existing tool name or alias", a)
			seen[a] = true
		}
	}
}

// TestStorageOnByDefault locks in the contract that storage is on by default (along with all groups)
// and is hidden when features is explicitly narrowed.
func TestStorageOnByDefault(t *testing.T) {
	storageNames := make([]string, 0)
	for _, s := range storageTools() {
		storageNames = append(storageNames, s.meta.name)
	}
	require.NotEmpty(t, storageNames)

	// Default (all groups on): storage tools are directly visible.
	def := listToolNames(t, connect(t, Options{}))
	for _, n := range storageNames {
		assert.True(t, def[n], "%s must be exposed by default", n)
	}
	assert.True(t, def["execute_sql"], "database tools should be on by default")

	// --features is replace-not-extend: keeping only database hides storage.
	narrowed := listToolNames(t, connect(t, Options{Features: []string{featureDatabase}}))
	for _, n := range storageNames {
		assert.False(t, narrowed[n], "%s must be hidden when features is narrowed", n)
	}
	assert.True(t, narrowed["execute_sql"])
}

// TestReadOnlyServerHidesAllMutatingTools end-to-end: no registered tool in read-only mode is a write tool.
func TestReadOnlyServerHidesAllMutatingTools(t *testing.T) {
	// Collect the names of all mutating tools from the full catalog.
	mutating := make(map[string]bool)
	for _, s := range allSpecs() {
		if s.meta.mutating {
			mutating[s.meta.name] = true
		}
	}
	require.NotEmpty(t, mutating, "catalog should contain mutating tools")

	// Default is all groups on (including storage); layering read-only confirms no write tool is exposed.
	names := listToolNames(t, connect(t, Options{ReadOnly: true}))
	for name := range names {
		assert.False(t, mutating[name], "mutating tool %q must be hidden in read-only mode", name)
	}
}
