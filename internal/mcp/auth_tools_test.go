// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthToolsMeta(t *testing.T) {
	specs := authTools()

	byName := make(map[string]meta, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s.meta
	}
	require.Len(t, specs, 8)

	want := map[string]bool{ // name -> mutating
		"get_auth_config":          false,
		"update_auth_config":       true,
		"get_auth_hooks_config":    false,
		"update_auth_hooks_config": true,
		"list_third_party_auth":    false,
		"create_third_party_auth":  true,
		"delete_third_party_auth":  true,
		"sync_third_party_auth":    true,
	}
	for name, mutating := range want {
		m, ok := byName[name]
		require.True(t, ok, "expected tool %s to be defined", name)
		assert.Equal(t, featureAuth, m.feature, "tool %s should belong to the auth feature", name)
		assert.Equal(t, mutating, m.mutating, "tool %s mutating flag mismatch", name)
	}
}

// The auth group is on by default and, like pages, is NOT hidden under --workspace-ref
// (its tools are branch-scoped and use the bound workspace). Read-only mode hides the
// mutating tools but keeps the read tools.
func TestAuthToolsExposedByDefaultAndUnderWorkspaceRef(t *testing.T) {
	def := listToolNames(t, connect(t, Options{}))
	assert.True(t, def["get_auth_config"], "auth tools must be on by default")
	assert.True(t, def["update_auth_config"])

	scoped := listToolNames(t, connect(t, Options{WorkspaceRef: "ws-123"}))
	assert.True(t, scoped["get_auth_config"], "auth tools stay visible under --workspace-ref")
	assert.True(t, scoped["update_auth_config"])

	// Narrowing features to another group hides the auth tools.
	narrowed := listToolNames(t, connect(t, Options{Features: []string{featureDatabase}}))
	assert.False(t, narrowed["get_auth_config"], "auth tools hidden when features is narrowed")

	// Read-only keeps the read tools, hides the mutating ones.
	ro := listToolNames(t, connect(t, Options{ReadOnly: true}))
	assert.True(t, ro["get_auth_config"], "read tool stays in read-only mode")
	assert.True(t, ro["list_third_party_auth"])
	assert.False(t, ro["update_auth_config"], "mutating tool hidden in read-only mode")
	assert.False(t, ro["create_third_party_auth"])
	assert.False(t, ro["sync_third_party_auth"])
}

// Required-argument validation runs before any client/credential resolution, so these
// reach an error without touching the network or triggering the login flow.

func TestUpdateAuthConfigRequiresConfig(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAuth}})
	_, err := updateAuthConfig(context.Background(), &p, updateAuthConfigInput{WorkspaceID: "ws-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestUpdateAuthHooksConfigRequiresConfig(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAuth}})
	_, err := updateAuthHooksConfig(context.Background(), &p, updateAuthHooksConfigInput{WorkspaceID: "ws-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestDeleteThirdPartyAuthRequiresProviderID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAuth}})
	_, err := deleteThirdPartyAuth(context.Background(), &p, deleteThirdPartyAuthInput{ProviderID: "  ", WorkspaceID: "ws-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider_id is required")
}

func TestGetAuthConfigRequiresWorkspace(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAuth}})
	_, err := getAuthConfig(context.Background(), &p, authTarget{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}
