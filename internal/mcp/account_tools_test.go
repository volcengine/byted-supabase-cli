// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func TestAccountGroupListed(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureAccount}})
	names := listToolNames(t, cs)
	for _, n := range []string{"list_workspaces", "get_workspace", "create_workspace", "stop_workspace", "start_workspace", "set_workspace_deletion_protection", "delete_workspace"} {
		assert.True(t, names[n], "expected %s to be listed", n)
	}
	// The old names are kept callable as aliases but must NOT be advertised in tools/list.
	assert.False(t, names["pause_workspace"], "pause_workspace alias must be hidden from the catalog")
	assert.False(t, names["restore_workspace"], "restore_workspace alias must be hidden from the catalog")
}

func TestAccountReadOnlyHidesMutating(t *testing.T) {
	cs := connect(t, Options{ReadOnly: true, Features: []string{featureAccount}})
	names := listToolNames(t, cs)
	// Read tools are kept.
	assert.True(t, names["list_workspaces"])
	assert.True(t, names["get_workspace"])
	// Write tools are hidden.
	assert.False(t, names["create_workspace"])
	assert.False(t, names["stop_workspace"])
	assert.False(t, names["start_workspace"])
	assert.False(t, names["set_workspace_deletion_protection"])
	assert.False(t, names["delete_workspace"])
}

// TestDeleteWorkspaceRequiresWorkspaceID verifies that, when not bound to a workspace,
// an empty workspace_id is rejected before any network call (preventing accidental
// deletion of a default or unspecified workspace).
func TestDeleteWorkspaceRequiresWorkspaceID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAccount}})
	_, err := deleteWorkspace(context.Background(), &p, workspaceTarget{WorkspaceID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}

// TestDeleteWorkspaceHiddenWhenWorkspaceScoped verifies that when the server is hard-scoped
// to a single workspace, the entire account group (including delete_workspace) is hidden,
// preventing deletion of other workspaces.
func TestDeleteWorkspaceHiddenWhenWorkspaceScoped(t *testing.T) {
	cs := connect(t, Options{WorkspaceRef: "ws-fixed", Features: []string{featureAccount, featureDatabase}})
	names := listToolNames(t, cs)
	assert.False(t, names["delete_workspace"], "delete_workspace must be hidden under workspace scope")
}

// TestSetDeletionProtectionRequiresEnabled 校验:漏传 enabled 时在任何网络调用前被拒绝,
// 不会因字段缺省被当成关闭保护。
func TestSetDeletionProtectionRequiresEnabled(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAccount}})
	_, err := setWorkspaceDeletionProtection(context.Background(), &p, setDeletionProtectionInput{WorkspaceID: "ws-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enabled is required")
}

// TestSetDeletionProtectionRequiresWorkspaceID 校验:enabled 已传但 workspace_id 为空时,
// 在任何网络调用前被拒绝。
func TestSetDeletionProtectionRequiresWorkspaceID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAccount}})
	enabled := false
	_, err := setWorkspaceDeletionProtection(context.Background(), &p, setDeletionProtectionInput{WorkspaceID: "  ", Enabled: &enabled})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace_id is required")
}

func TestAccountGroupHiddenWhenWorkspaceScoped(t *testing.T) {
	// When the server is hard-scoped to a single workspace, account-level management is not exposed at all.
	cs := connect(t, Options{WorkspaceRef: "ws-fixed", Features: []string{featureAccount, featureDatabase}})
	names := listToolNames(t, cs)
	for _, n := range []string{"list_workspaces", "get_workspace", "create_workspace", "stop_workspace", "start_workspace", "set_workspace_deletion_protection", "delete_workspace"} {
		assert.False(t, names[n], "expected %s to be hidden under workspace scope", n)
	}
	// Other groups are still available.
	assert.True(t, names["execute_sql"])
}

// TestToWorkspaceViewAgentPlanFields verifies that agent-plan fields are correctly
// projected from volcengine.Workspace into the client-facing view.
func TestToWorkspaceViewAgentPlanFields(t *testing.T) {
	agentPlan := toWorkspaceView(volcengine.Workspace{
		WorkspaceID:         "ws-1",
		IsAgentPlan:         true,
		IsAgentPlanInstance: true,
		AgentPlanSeatID:     "seat-123",
	})
	assert.True(t, agentPlan.IsAgentPlan)
	assert.True(t, agentPlan.IsAgentPlanInstance)
	assert.Equal(t, "seat-123", agentPlan.AgentPlanSeatID)

	normal := toWorkspaceView(volcengine.Workspace{WorkspaceID: "ws-2"})
	assert.False(t, normal.IsAgentPlan)
	assert.False(t, normal.IsAgentPlanInstance)
	assert.Empty(t, normal.AgentPlanSeatID)
}

// TestResolveAgentPlan verifies the server-side Agent Plan default (env AGENT_PLAN /
// AGENT_PLAN_SEAT_ID): it applies only when the create call omitted every agent-plan
// field; explicit caller input — including is_agent_plan=false — always wins.
func TestResolveAgentPlan(t *testing.T) {
	ptr := func(b bool) *bool { return &b }

	// Isolate profile + credential state so the caller/server-only cases never pick up a
	// persisted profile default from the developer's real ~/.volcengine config.
	t.Setenv("HOME", t.TempDir())
	t.Setenv(volcengine.EnvAccessKeyID, "")
	t.Setenv(volcengine.EnvSecretAccessKey, "")
	t.Setenv(volcengine.EnvSessionToken, "")

	cases := []struct {
		name     string
		in       createWorkspaceInput
		opts     Options
		wantPlan *bool
		wantSeat string
	}{
		{name: "no default, no input → ordinary"},
		{name: "personal default applies when caller silent", opts: Options{AgentPlan: true}, wantPlan: ptr(true)},
		{name: "seat default applies when caller silent", opts: Options{AgentPlanSeatID: "seat-x"}, wantSeat: "seat-x"},
		{name: "seat default wins over personal default", opts: Options{AgentPlan: true, AgentPlanSeatID: "seat-x"}, wantSeat: "seat-x"},
		{name: "explicit true wins over no default", in: createWorkspaceInput{IsAgentPlan: ptr(true)}, wantPlan: ptr(true)},
		{name: "explicit false opts out of personal default", in: createWorkspaceInput{IsAgentPlan: ptr(false)}, opts: Options{AgentPlan: true}, wantPlan: ptr(false)},
		{name: "explicit false opts out of seat default", in: createWorkspaceInput{IsAgentPlan: ptr(false)}, opts: Options{AgentPlanSeatID: "seat-x"}, wantPlan: ptr(false)},
		{name: "explicit seat wins over default seat", in: createWorkspaceInput{AgentPlanSeatID: "seat-caller"}, opts: Options{AgentPlanSeatID: "seat-env"}, wantSeat: "seat-caller"},
		{name: "default seat trimmed", opts: Options{AgentPlanSeatID: "  seat-y  "}, wantSeat: "seat-y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newPolicy(tc.opts)
			gotPlan, gotSeat, err := resolveAgentPlan(tc.in, &p)
			require.NoError(t, err)
			assert.Equal(t, tc.wantSeat, gotSeat)
			if tc.wantPlan == nil {
				assert.Nil(t, gotPlan)
			} else if assert.NotNil(t, gotPlan) {
				assert.Equal(t, *tc.wantPlan, *gotPlan)
			}
		})
	}
}

// TestResolveAgentPlanProfileDefault verifies that create_workspace honors the Agent Plan
// default persisted by `login` / `configure ... --is-agent-plan[ --agent-plan-seat-id]`
// when the caller and server are both silent — and that it mirrors the CLI's credential
// gating (env-AK/SK auth skips the profile default).
func TestResolveAgentPlanProfileDefault(t *testing.T) {
	ptr := func(b bool) *bool { return &b }

	// setupProfile isolates HOME, clears credential env vars (so a persistent profile is in
	// effect), and persists the given Agent Plan default on the active ("default") profile.
	setupProfile := func(t *testing.T, def volcengine.SupabaseProfileConfig) {
		t.Helper()
		t.Setenv("HOME", t.TempDir())
		t.Setenv(volcengine.EnvAccessKeyID, "")
		t.Setenv(volcengine.EnvSecretAccessKey, "")
		t.Setenv(volcengine.EnvSessionToken, "")
		require.NoError(t, volcengine.SaveFileConfig(volcengine.FileConfig{
			Current:  "default",
			Profiles: map[string]*volcengine.Profile{"default": {Name: "default", Mode: "ak"}},
		}))
		require.NoError(t, volcengine.SetSupabaseProfileConfig("default", def))
	}

	t.Run("enterprise seat default applies when caller and server silent", func(t *testing.T) {
		setupProfile(t, volcengine.SupabaseProfileConfig{AgentPlanSeatID: "seat-profile"})
		p := newPolicy(Options{})
		plan, seat, err := resolveAgentPlan(createWorkspaceInput{}, &p)
		require.NoError(t, err)
		assert.Equal(t, "seat-profile", seat)
		require.NotNil(t, plan)
		assert.True(t, *plan)
	})

	t.Run("personal default applies when caller and server silent", func(t *testing.T) {
		setupProfile(t, volcengine.SupabaseProfileConfig{IsAgentPlan: true})
		p := newPolicy(Options{})
		plan, seat, err := resolveAgentPlan(createWorkspaceInput{}, &p)
		require.NoError(t, err)
		assert.Empty(t, seat)
		require.NotNil(t, plan)
		assert.True(t, *plan)
	})

	t.Run("server default overrides profile default", func(t *testing.T) {
		setupProfile(t, volcengine.SupabaseProfileConfig{AgentPlanSeatID: "seat-profile"})
		p := newPolicy(Options{AgentPlan: true})
		plan, seat, err := resolveAgentPlan(createWorkspaceInput{}, &p)
		require.NoError(t, err)
		assert.Empty(t, seat) // the profile seat is ignored: the server default wins
		require.NotNil(t, plan)
		assert.True(t, *plan)
	})

	t.Run("explicit caller opt-out overrides profile default", func(t *testing.T) {
		setupProfile(t, volcengine.SupabaseProfileConfig{AgentPlanSeatID: "seat-profile"})
		p := newPolicy(Options{})
		plan, seat, err := resolveAgentPlan(createWorkspaceInput{IsAgentPlan: ptr(false)}, &p)
		require.NoError(t, err)
		assert.Empty(t, seat)
		require.NotNil(t, plan)
		assert.False(t, *plan)
	})

	t.Run("env credential auth skips profile default (CLI parity)", func(t *testing.T) {
		setupProfile(t, volcengine.SupabaseProfileConfig{AgentPlanSeatID: "seat-profile"})
		t.Setenv(volcengine.EnvAccessKeyID, "env-ak")
		t.Setenv(volcengine.EnvSecretAccessKey, "env-sk")
		p := newPolicy(Options{})
		plan, seat, err := resolveAgentPlan(createWorkspaceInput{}, &p)
		require.NoError(t, err)
		assert.Nil(t, plan)
		assert.Empty(t, seat)
	})
}

func TestCreateWorkspaceRequiresName(t *testing.T) {
	cs := connect(t, Options{Features: []string{featureAccount}})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_workspace",
		Arguments: map[string]any{"workspace_name": "  "},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "workspace_name is required")
}

// TestCreateWorkspaceRejectsInvalidSuspend verifies that an invalid suspend timeout (in the
// 1..299 range) passed at creation is rejected before any API call.
func TestCreateWorkspaceRejectsInvalidSuspend(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAccount}})
	bad := 150
	_, err := createWorkspace(context.Background(), &p, createWorkspaceInput{
		WorkspaceName:         "ws-test",
		SuspendTimeoutSeconds: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid suspend_timeout_seconds")
}
