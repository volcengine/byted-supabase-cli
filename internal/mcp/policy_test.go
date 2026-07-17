// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPolicy is the single construction point for the access policy; the cases below
// lock in the default-fill, trim, and deduplication behaviour.
func TestNewPolicyDefaults(t *testing.T) {
	// Empty features falls back to the default set (all official groups, including storage).
	p := newPolicy(Options{})
	for _, f := range defaultFeatures() {
		assert.True(t, p.features[f], "default policy should enable %s", f)
	}
	assert.True(t, p.features[featureStorage], "storage must be on by default")
	assert.False(t, p.readOnly)
	assert.Empty(t, p.workspaceRef)
	assert.Empty(t, p.disabledTools)
}

func TestNewPolicyTrimsAndDropsBlanks(t *testing.T) {
	p := newPolicy(Options{
		Features:      []string{" storage ", "", "   ", "database"},
		DisabledTools: []string{" execute_sql ", ""},
		WorkspaceRef:  "  ws-fixed  ",
		ReadOnly:      true,
	})
	// Non-empty features no longer falls back to the default: only explicitly provided groups (after trimming) are kept.
	assert.True(t, p.features["storage"])
	assert.True(t, p.features["database"])
	assert.Len(t, p.features, 2, "blank feature entries must be dropped")
	// The denylist and workspaceRef are trimmed in the same way.
	assert.True(t, p.disabledTools["execute_sql"])
	assert.Len(t, p.disabledTools, 1)
	assert.Equal(t, "ws-fixed", p.workspaceRef)
	assert.True(t, p.readOnly)
}

// validateOptions is the startup fail-fast gate: unknown names in features or disabled-tools cause an error.
func TestValidateOptions(t *testing.T) {
	t.Run("empty is valid", func(t *testing.T) {
		require.NoError(t, validateOptions(Options{}))
	})
	t.Run("implemented and legacy features pass", func(t *testing.T) {
		require.NoError(t, validateOptions(Options{
			Features: []string{featureStorage, "database", "docs", "debugging", "  ", ""},
		}))
	})
	t.Run("real tool names pass disabled-tools", func(t *testing.T) {
		require.NoError(t, validateOptions(Options{
			DisabledTools: []string{"execute_sql", "health_check"},
		}))
	})
	t.Run("unknown feature is rejected", func(t *testing.T) {
		err := validateOptions(Options{Features: []string{"database", "bogus"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported features:")
		assert.Contains(t, err.Error(), "bogus")
	})
	t.Run("unknown disabled-tool is rejected", func(t *testing.T) {
		err := validateOptions(Options{DisabledTools: []string{"execute_sql", "not_a_tool"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported disabled-tools:")
		assert.Contains(t, err.Error(), "not_a_tool")
	})
	t.Run("bad names are trimmed, deduped and sorted", func(t *testing.T) {
		err := validateOptions(Options{Features: []string{" zeta ", "alpha", "zeta", "alpha"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported features: alpha, zeta")
	})
}

// allows is the core tool-exposure decision: denylist > read-only > unconditional > account-scoped > feature-gated.
func TestPolicyAllows(t *testing.T) {
	cases := []struct {
		name     string
		p        policy
		tool     string
		feature  string
		mutating bool
		want     bool
	}{
		{
			name:    "enabled feature read tool",
			p:       newPolicy(Options{Features: []string{featureDatabase}}),
			tool:    "list_tables",
			feature: featureDatabase,
			want:    true,
		},
		{
			name:    "disabled feature hidden",
			p:       newPolicy(Options{Features: []string{featureDatabase}}),
			tool:    "list_branches",
			feature: featureBranching,
			want:    false,
		},
		{
			name:     "blacklist wins over enabled feature",
			p:        newPolicy(Options{Features: []string{featureDatabase}, DisabledTools: []string{"list_tables"}}),
			tool:     "list_tables",
			feature:  featureDatabase,
			mutating: false,
			want:     false,
		},
		{
			name:     "read-only hides mutating tool",
			p:        newPolicy(Options{ReadOnly: true, Features: []string{featureDatabase}}),
			tool:     "execute_sql",
			feature:  featureDatabase,
			mutating: true,
			want:     false,
		},
		{
			name:     "read-only keeps read tool",
			p:        newPolicy(Options{ReadOnly: true, Features: []string{featureDatabase}}),
			tool:     "list_tables",
			feature:  featureDatabase,
			mutating: false,
			want:     true,
		},
		{
			name:    "empty feature is unconditional",
			p:       newPolicy(Options{Features: []string{featureStorage}}),
			tool:    "health_check",
			feature: "",
			want:    true,
		},
		{
			name:     "empty feature mutating still hidden in read-only",
			p:        newPolicy(Options{ReadOnly: true, Features: []string{featureStorage}}),
			tool:     "hypothetical_write",
			feature:  "",
			mutating: true,
			want:     false,
		},
		{
			name:    "empty feature still removable by blacklist",
			p:       newPolicy(Options{DisabledTools: []string{"health_check"}}),
			tool:    "health_check",
			feature: "",
			want:    false,
		},
		{
			name:    "account hidden when workspace-scoped",
			p:       newPolicy(Options{WorkspaceRef: "ws-fixed", Features: []string{featureAccount}}),
			tool:    "list_workspaces",
			feature: featureAccount,
			want:    false,
		},
		{
			name:    "account shown without workspace scope",
			p:       newPolicy(Options{Features: []string{featureAccount}}),
			tool:    "list_workspaces",
			feature: featureAccount,
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.p.allows(tc.tool, tc.feature, tc.mutating))
		})
	}
}

func TestPolicyEnabledFeatures(t *testing.T) {
	// The default set is sorted and includes all official groups (including storage).
	def := newPolicy(Options{}).enabledFeatures()
	assert.Equal(t, []string{
		featureAccount, featureAuth, featureBranching, featureCompute, featureDatabase,
		featureDevelopment, featureFunctions, featurePages, featureStorage,
	}, def)
	assert.Contains(t, def, featureStorage)

	// Unrecognised feature names do not appear in the result (only official groups are returned), and the result is sorted.
	custom := newPolicy(Options{Features: []string{"storage", "bogus", "database"}}).enabledFeatures()
	assert.Equal(t, []string{featureDatabase, featureStorage}, custom)
}

// resolveWorkspace is both the enforcement point for workspace-ref hard-binding and the error point when workspace_id is missing.
func TestPolicyResolveWorkspace(t *testing.T) {
	scoped := newPolicy(Options{WorkspaceRef: "ws-fixed"})
	free := newPolicy(Options{})

	t.Run("scoped ignores empty input", func(t *testing.T) {
		got, err := scoped.resolveWorkspace("")
		require.NoError(t, err)
		assert.Equal(t, "ws-fixed", got)
	})
	t.Run("scoped accepts matching input", func(t *testing.T) {
		got, err := scoped.resolveWorkspace("  ws-fixed  ")
		require.NoError(t, err)
		assert.Equal(t, "ws-fixed", got)
	})
	t.Run("scoped rejects mismatched input", func(t *testing.T) {
		_, err := scoped.resolveWorkspace("ws-other")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not allowed")
		assert.Contains(t, err.Error(), "ws-fixed")
	})
	t.Run("free requires input", func(t *testing.T) {
		_, err := free.resolveWorkspace("   ")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workspace_id is required")
	})
	t.Run("free trims input", func(t *testing.T) {
		got, err := free.resolveWorkspace("  ws-1  ")
		require.NoError(t, err)
		assert.Equal(t, "ws-1", got)
	})
}
