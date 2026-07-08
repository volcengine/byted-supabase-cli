// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupabaseProfileConfigRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, SetSupabaseProfileConfig("default", SupabaseProfileConfig{
		IsAgentPlan:     true,
		AgentPlanSeatID: " seat-123 ",
	}))
	cfg, err := LoadSupabaseFileConfig()
	require.NoError(t, err)
	assert.Equal(t, SupabaseProfileConfig{
		IsAgentPlan:     true,
		AgentPlanSeatID: "seat-123",
	}, cfg["default"])

	path, err := SupabaseConfigFilePath()
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	dirInfo, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm())
	assert.Equal(t, filepath.Join(os.Getenv("HOME"), ".volcengine", supabaseConfigDirectory, "supabase_config.json"), path)

	require.NoError(t, DeleteSupabaseProfileConfig("default"))
	cfg, err = LoadSupabaseFileConfig()
	require.NoError(t, err)
	assert.NotContains(t, cfg, "default")
}

func TestResolveCreateWorkspaceAgentPlan(t *testing.T) {
	oldProfile := profileOverride
	t.Cleanup(func() {
		profileOverride = oldProfile
	})
	profileOverride = ""
	t.Setenv("HOME", t.TempDir())
	t.Setenv(EnvAccessKeyID, "")
	t.Setenv(EnvSecretAccessKey, "")
	t.Setenv(EnvSessionToken, "")

	require.NoError(t, SaveFileConfig(FileConfig{
		Current: "enterprise",
		Profiles: map[string]*Profile{
			"enterprise": {Name: "enterprise", Mode: ModeAK, AccessKey: "ak", SecretKey: "sk"},
		},
	}))
	require.NoError(t, SetSupabaseProfileConfig("enterprise", SupabaseProfileConfig{
		IsAgentPlan:     true,
		AgentPlanSeatID: "seat-123",
	}))

	resolved, err := ResolveCreateWorkspaceAgentPlan(CreateWorkspaceParams{})
	require.NoError(t, err)
	require.NotNil(t, resolved.IsAgentPlan)
	assert.True(t, *resolved.IsAgentPlan)
	assert.Equal(t, "seat-123", resolved.AgentPlanSeatID)

	disabled := false
	resolved, err = ResolveCreateWorkspaceAgentPlan(CreateWorkspaceParams{IsAgentPlan: &disabled})
	require.NoError(t, err)
	require.NotNil(t, resolved.IsAgentPlan)
	assert.False(t, *resolved.IsAgentPlan)
	assert.Empty(t, resolved.AgentPlanSeatID)

	_, err = ResolveCreateWorkspaceAgentPlan(CreateWorkspaceParams{
		IsAgentPlan:     &disabled,
		AgentPlanSeatID: "seat-123",
	})
	require.Error(t, err)

	t.Setenv(EnvAccessKeyID, "env-ak")
	t.Setenv(EnvSecretAccessKey, "env-sk")
	resolved, err = ResolveCreateWorkspaceAgentPlan(CreateWorkspaceParams{})
	require.NoError(t, err)
	assert.Nil(t, resolved.IsAgentPlan)
	assert.Empty(t, resolved.AgentPlanSeatID)
}
