// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevelopmentToolsMeta(t *testing.T) {
	specs := developmentTools()
	byName := make(map[string]toolSpec, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s
	}

	expected := []string{"generate_typescript_types", "get_workspace_url", "get_publishable_keys", "list_workspace_operations"}
	assert.Len(t, specs, len(expected))
	for _, name := range expected {
		s, ok := byName[name]
		require.True(t, ok, "expected %s to be defined", name)
		assert.Equal(t, featureDevelopment, s.meta.feature, "%s must be in the development feature group", name)
		assert.False(t, s.meta.mutating, "%s must be read-only", name)
	}
}

// The following tests cover only the offline validation path: each tool resolves the
// workspace before making any network call, so the handler returns early when
// workspace_id is missing and no server-level binding is set.

func TestGenerateTypescriptTypesRequiresWorkspace(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureDevelopment}})
	_, err := generateTypescriptTypes(context.Background(), &p, generateTypescriptTypesInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}

func TestGenerateTypescriptTypesRejectsInvalidSchema(t *testing.T) {
	// Schema names are validated before workspace resolution.
	p := newPolicy(Options{Features: []string{featureDevelopment}})
	_, err := generateTypescriptTypes(context.Background(), &p, generateTypescriptTypesInput{
		Schemas: []string{"public; drop table x"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid schema name")
}

func TestGetWorkspaceURLRequiresWorkspace(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureDevelopment}})
	_, err := getWorkspaceURL(context.Background(), &p, developmentTarget{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}

func TestGetPublishableKeysRequiresWorkspace(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureDevelopment}})
	_, err := getPublishableKeys(context.Background(), &p, getPublishableKeysInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}

func TestListWorkspaceOperationsRequiresWorkspace(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureDevelopment}})
	_, err := listWorkspaceOperations(context.Background(), &p, listWorkspaceOperationsInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}

func TestMaskKey(t *testing.T) {
	// With reveal=true the value is returned unchanged.
	assert.Equal(t, "supersecretkey1234", maskKey("supersecretkey1234", true))
	// An empty value stays empty regardless of reveal.
	assert.Equal(t, "", maskKey("", false))
	assert.Equal(t, "", maskKey("", true))
	// Short values (≤12 chars) are fully masked.
	assert.Equal(t, "************", maskKey("123456789012", false))
	assert.Equal(t, "*****", maskKey("short", false))
	// Longer values keep the first 6 and last 4 chars with an ellipsis in between.
	assert.Equal(t, "sbp_12...cdef", maskKey("sbp_1234567890abcdef", false))
}
