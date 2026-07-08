// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionsToolsMeta(t *testing.T) {
	specs := functionsTools()

	byName := make(map[string]meta, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s.meta
	}
	require.Len(t, specs, 4)

	want := map[string]bool{ // name -> mutating
		"list_edge_functions":  false,
		"get_edge_function":    false,
		"deploy_edge_function": true,
		"delete_edge_function": true,
	}
	for name, mutating := range want {
		m, ok := byName[name]
		require.True(t, ok, "expected tool %s to be defined", name)
		assert.Equal(t, featureFunctions, m.feature, "tool %s should belong to the functions feature", name)
		assert.Equal(t, mutating, m.mutating, "tool %s mutating flag mismatch", name)
	}
}

func TestGetEdgeFunctionRequiresSlug(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := getEdgeFunction(context.Background(), &p, getEdgeFunctionInput{Slug: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestDeleteEdgeFunctionRequiresSlug(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := deleteEdgeFunction(context.Background(), &p, deleteEdgeFunctionInput{Slug: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestDeployEdgeFunctionRequiresSlug(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := deployEdgeFunction(context.Background(), &p, deployEdgeFunctionInput{Slug: "", Code: "console.log('hi')"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "slug is required")
}

func TestDeployEdgeFunctionRequiresCode(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := deployEdgeFunction(context.Background(), &p, deployEdgeFunctionInput{Slug: "hello", Code: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "code is required")
}

func TestDeployEdgeFunctionRejectsUnsupportedRuntime(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := deployEdgeFunction(context.Background(), &p, deployEdgeFunctionInput{
		Slug:    "hello",
		Code:    "console.log('hi')",
		Runtime: "native-bun/v1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

// An invalid slug (contains a space, violates FuncSlugPattern) is rejected before any network call.
func TestDeployEdgeFunctionRejectsInvalidSlug(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := deployEdgeFunction(context.Background(), &p, deployEdgeFunctionInput{
		Slug: "bad slug",
		Code: "console.log('hi')",
	})
	require.Error(t, err)
}

// An invalid import_map JSON string is intercepted before being sent to the gateway (validation runs before writeClientFor).
func TestDeployEdgeFunctionRejectsInvalidImportMap(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureFunctions}})
	_, err := deployEdgeFunction(context.Background(), &p, deployEdgeFunctionInput{
		Slug:      "hello",
		Code:      "console.log('hi')",
		ImportMap: "{not valid json",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "import_map must be valid JSON")
}
