// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// accountTools registers the account group: workspace (project) management, calling volcengine.Client directly.
// The entire group is hidden when the server is scoped to a single workspace (see policy.allows).
func accountTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "list_workspaces",
			title:       "List workspaces",
			feature:     featureAccount,
			description: "List Supabase workspaces (projects) in the configured region.",
		}, listWorkspaces),
		defineTool(meta{
			name:        "get_workspace",
			title:       "Get workspace",
			feature:     featureAccount,
			description: "Get details of a single Supabase workspace.",
		}, getWorkspace),
		defineTool(meta{
			name:        "create_workspace",
			title:       "Create workspace",
			feature:     featureAccount,
			mutating:    true,
			description: "Create a new Supabase workspace. By default auto-suspend is disabled (suspend_timeout_seconds = -1, i.e. compute stays always-on / never suspends); pass suspend_timeout_seconds to set a different value at creation (>=300 to enable auto-suspend, -1 to keep always-on). For Agent Plan, pass is_agent_plan=true without agent_plan_seat_id for personal edition, or pass agent_plan_seat_id for enterprise edition (it implies is_agent_plan=true). Omit both Agent Plan fields to use the current persistent profile default; environment authentication does not use profile defaults. After creation, proactively explain in one line what the auto-suspend idle timeout means and non-blockingly ask whether to set one — recommend 3600s (60 min) to save compute; if the user declines or does not answer, keep the always-on default and do NOT block. It can also be changed later with modify_compute_settings (suspend_timeout_seconds).",
		}, createWorkspace),
		defineTool(meta{
			name:        "stop_workspace",
			title:       "Stop workspace",
			aliases:     []string{"pause_workspace"},
			feature:     featureAccount,
			mutating:    true,
			description: "Stop (pause) a running Supabase workspace, matching the CLI `projects stop`. Only works when the workspace is in the Running state.",
		}, stopWorkspace),
		defineTool(meta{
			name:        "start_workspace",
			title:       "Start workspace",
			aliases:     []string{"restore_workspace"},
			feature:     featureAccount,
			mutating:    true,
			description: "Start (restore) a Supabase workspace that is paused (Stopped) or auto-suspended (Suspended), matching the CLI `projects start`.",
		}, startWorkspace),
		defineTool(meta{
			name:        "set_workspace_deletion_protection",
			title:       "Set workspace deletion protection",
			feature:     featureAccount,
			mutating:    true,
			description: "Enable or disable deletion protection on a Supabase workspace. Deletion protection must be disabled (enabled=false) before delete_workspace can succeed; use this first when a delete is blocked by protection. Set enabled=true to re-protect a workspace.",
		}, setWorkspaceDeletionProtection),
		defineTool(meta{
			name:        "delete_workspace",
			title:       "Delete workspace",
			feature:     featureAccount,
			mutating:    true,
			description: "Permanently delete a Supabase workspace. Irreversible. Fails if deletion protection is enabled (it is enabled by default on new workspaces); disable it first with set_workspace_deletion_protection (enabled=false).",
		}, deleteWorkspace),
	}
}

// workspaceView is the slim projection returned to the client.
type workspaceView struct {
	WorkspaceID              string `json:"workspace_id"`
	WorkspaceName            string `json:"workspace_name"`
	Status                   string `json:"status"`
	Region                   string `json:"region"`
	EngineType               string `json:"engine_type,omitempty"`
	EngineVersion            string `json:"engine_version,omitempty"`
	DeletionProtectionStatus string `json:"deletion_protection_status,omitempty"`
	CreatedAt                string `json:"created_at,omitempty"`
	UpdatedAt                string `json:"updated_at,omitempty"`
	// SuspendTimeoutSeconds is the auto-suspend idle timeout in seconds: >=300 suspends after
	// that many idle seconds; -1 disables auto-suspend (always-on / never suspend, the create
	// default); 0 means unset (platform default, behaves as always-on). Only backfilled for
	// get/create single-instance views (create echoes the value just sent; get reads it from
	// workspace detail ComputeSettings); omitted from list views to avoid misreading it as 0.
	SuspendTimeoutSeconds *int `json:"suspend_timeout_seconds,omitempty"`
	// Agent-plan fields: false/empty for ordinary instances; agent-plan instances carry seat info.
	IsAgentPlan         bool   `json:"is_agent_plan"`
	IsAgentPlanInstance bool   `json:"is_agent_plan_instance"`
	AgentPlanSeatID     string `json:"agent_plan_seat_id,omitempty"`
}

func toWorkspaceView(w volcengine.Workspace) workspaceView {
	return workspaceView{
		WorkspaceID:              w.WorkspaceID,
		WorkspaceName:            w.WorkspaceName,
		Status:                   w.WorkspaceStatus,
		Region:                   w.RegionID,
		EngineType:               w.EngineType,
		EngineVersion:            w.EngineVersion,
		DeletionProtectionStatus: w.DeletionProtectionStatus,
		CreatedAt:                w.CreateTime,
		UpdatedAt:                w.UpdateTime,
		IsAgentPlan:              w.IsAgentPlan,
		IsAgentPlanInstance:      w.IsAgentPlanInstance,
		AgentPlanSeatID:          w.AgentPlanSeatID,
	}
}

// workspaceTarget identifies a single workspace.
type workspaceTarget struct {
	WorkspaceID string `json:"workspace_id" jsonschema:"workspace (project) id"`
}

type listWorkspacesInput struct {
	ProjectName string `json:"project_name,omitempty" jsonschema:"filter by project name"`
	Count       int    `json:"count,omitempty" jsonschema:"maximum number of workspaces to return per page; defaults to 10"`
	Offset      int    `json:"offset,omitempty" jsonschema:"pagination offset (number of workspaces to skip); defaults to 0"`
}

func listWorkspaces(ctx context.Context, p *policy, in listWorkspacesInput) (string, error) {
	client, err := p.readClient(ctx)
	if err != nil {
		return "", err
	}
	result, err := client.ListWorkspaces(ctx, volcengine.ListWorkspacesParams{
		ProjectName: strings.TrimSpace(in.ProjectName),
		Limit:       listLimit(in.Count),
		Offset:      in.Offset,
	})
	if err != nil {
		return "", err
	}
	views := make([]workspaceView, 0, len(result.Workspaces))
	for _, w := range result.Workspaces {
		views = append(views, toWorkspaceView(w))
	}
	return toJSON(map[string]any{
		"workspaces": views,
		"count":      len(views),
		"total":      result.Total,
	})
}

func getWorkspace(ctx context.Context, p *policy, in workspaceTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeWorkspaceDetail(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if result.Workspace.WorkspaceID == "" {
		return "", errors.Errorf("workspace %s not found", workspaceID)
	}
	view := toWorkspaceView(result.Workspace)
	suspend := result.Workspace.ComputeSettings.SuspendTimeoutSeconds
	view.SuspendTimeoutSeconds = &suspend
	return toJSON(view)
}

type createWorkspaceInput struct {
	WorkspaceName         string `json:"workspace_name" jsonschema:"name for the new workspace"`
	ProjectName           string `json:"project_name,omitempty" jsonschema:"optional project name to group the workspace under"`
	IsAgentPlan           *bool  `json:"is_agent_plan,omitempty" jsonschema:"optional whether to create an Agent Plan workspace; true without agent_plan_seat_id creates personal edition; omit both Agent Plan fields to use the current persistent profile default"`
	AgentPlanSeatID       string `json:"agent_plan_seat_id,omitempty" jsonschema:"optional Agent Plan seat id for enterprise edition; when set, is_agent_plan is treated as true automatically; omit both Agent Plan fields to use the current persistent profile default"`
	SuspendTimeoutSeconds *int   `json:"suspend_timeout_seconds,omitempty" jsonschema:"auto-suspend (scale-to-zero) idle timeout for the new workspace: >=300 = suspend after that many idle seconds; -1 = disable auto-suspend (always-on / never suspend). Omit to default to -1 (always-on). Recommend 3600 (60 min) to save compute"`
}

func createWorkspace(ctx context.Context, p *policy, in createWorkspaceInput) (string, error) {
	name := strings.TrimSpace(in.WorkspaceName)
	if name == "" {
		return "", errors.New("workspace_name is required")
	}
	// New workspaces explicitly default to auto-suspend disabled (-1 = always-on), avoiding the
	// easily-misread 0 (the unset default); callers may override with an explicit value.
	suspend := suspendTimeoutDisabled
	if in.SuspendTimeoutSeconds != nil {
		if err := validateSuspendTimeoutSeconds(*in.SuspendTimeoutSeconds); err != nil {
			return "", err
		}
		suspend = *in.SuspendTimeoutSeconds
	}
	isAgentPlan, seatID, err := resolveAgentPlan(in, p)
	if err != nil {
		return "", err
	}
	client, err := p.writeClient(ctx)
	if err != nil {
		return "", err
	}
	// Gate creation on the AIDAP service-linked role: without it the account cannot
	// use AIDAP, so block with the authorization guidance instead of creating.
	if err := client.CheckAIDAPServiceLinkedRole(); err != nil {
		return "", err
	}
	result, err := client.CreateSupabaseWorkspace(ctx, volcengine.CreateWorkspaceParams{
		WorkspaceName:         name,
		ProjectName:           strings.TrimSpace(in.ProjectName),
		IsAgentPlan:           isAgentPlan,
		AgentPlanSeatID:       seatID,
		SuspendTimeoutSeconds: &suspend,
	})
	if err != nil {
		return "", err
	}
	view := toWorkspaceView(result.Workspace)
	// Echo the value we just sent: the suspend value lands in the BaaS settings, whereas
	// result's ComputeSettings is the Database service (reading it would yield 0 — a known
	// display quirk), so we don't rely on the response and echo the applied value directly.
	view.SuspendTimeoutSeconds = &suspend
	return toJSON(map[string]any{
		"success":      true,
		"workspace_id": result.WorkspaceID,
		"workspace":    view,
		"suspend_hint": "新建实例默认关闭自动休眠(suspend_timeout_seconds = -1,即算力常驻、永不自动休眠)。请主动、非阻塞地问用户是否开启自动休眠,可直接用这段话:「关于空闲自动休眠:当前算力常驻、不会自动休眠——稳定但更耗资源。如果想节省算力,可配置空闲自动休眠时间(再次访问无需手动操作,Supabase 自动唤醒),推荐 1 小时。」用户同意后,调用一次 modify_compute_settings,只传 workspace_id 和 suspend_timeout_seconds=3600 即可(休眠超时是 workspace 级设置,无需 compute_id);如需之后再次关闭自动休眠,传 suspend_timeout_seconds=-1(算力常驻);用户拒绝或不回答则保持常驻默认,不要阻塞。",
	})
}

// resolveAgentPlan decides the effective Agent Plan parameters for a new workspace,
// in strict precedence order:
//
//  1. Explicit caller input — if the create_workspace call set is_agent_plan (including
//     false) or a seat id, that choice is used verbatim and every default below is ignored.
//  2. Server-configured default — the --agent-plan / --agent-plan-seat-id flags (env
//     AGENT_PLAN / AGENT_PLAN_SEAT_ID); a configured seat id (enterprise) wins over the
//     personal boolean.
//  3. Persisted profile default — the flag recorded by `login` / `configure ...
//     --is-agent-plan[ --agent-plan-seat-id]`. It is resolved through the shared
//     volcengine.ResolveCreateWorkspaceAgentPlan so MCP and the CLI behave identically:
//     the default applies only when authenticating via a persistent profile; env AK/SK
//     and non-persistent backend auth (e.g. proxy tokens) skip it.
//
// A (nil, "") result means an ordinary (non-Agent-Plan) instance — no caller, server, or
// profile default was set. CreateSupabaseWorkspace re-applies step 3 idempotently
// downstream, so resolving it here is purely to make the precedence explicit and testable.
func resolveAgentPlan(in createWorkspaceInput, p *policy) (*bool, string, error) {
	isAgentPlan := in.IsAgentPlan
	seatID := strings.TrimSpace(in.AgentPlanSeatID)
	if isAgentPlan != nil || seatID != "" {
		return isAgentPlan, seatID, nil
	}
	if p.agentPlanSeatID != "" {
		return nil, p.agentPlanSeatID, nil
	}
	if p.agentPlanDefault {
		on := true
		return &on, "", nil
	}
	// Caller and server are both silent: fall back to the persisted profile default,
	// resolved (and credential-gated) by the same helper the CLI's create path uses.
	resolved, err := volcengine.ResolveCreateWorkspaceAgentPlan(volcengine.CreateWorkspaceParams{})
	if err != nil {
		return nil, "", err
	}
	return resolved.IsAgentPlan, resolved.AgentPlanSeatID, nil
}

func stopWorkspace(ctx context.Context, p *policy, in workspaceTarget) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if err := client.StopWorkspace(ctx, workspaceID); err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "workspace_id": workspaceID, "action": "stop"})
}

func startWorkspace(ctx context.Context, p *policy, in workspaceTarget) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if err := client.StartWorkspace(ctx, workspaceID); err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "workspace_id": workspaceID, "action": "start"})
}

// setDeletionProtectionInput 切换 workspace 删除保护开关。enabled 用指针区分"未传"与 false,
// 强制调用方显式表态(避免漏传字段被当成关闭保护)。
type setDeletionProtectionInput struct {
	WorkspaceID string `json:"workspace_id" jsonschema:"workspace (project) id"`
	Enabled     *bool  `json:"enabled" jsonschema:"true to enable deletion protection; false to disable it (required before delete_workspace)"`
}

// setWorkspaceDeletionProtection 开/关 workspace 删除保护。删除受保护的 workspace 会被服务端拒绝,
// 删除前需先用本工具关闭保护(enabled=false)。补齐 delete_workspace 在保护开启时无法经 MCP 清理的缺口。
func setWorkspaceDeletionProtection(ctx context.Context, p *policy, in setDeletionProtectionInput) (string, error) {
	if in.Enabled == nil {
		return "", errors.New("enabled is required: set false to disable deletion protection (allowing delete), true to enable it")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if err := client.ModifyWorkspaceDeletionProtectionPolicy(ctx, workspaceID, *in.Enabled); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":             true,
		"workspace_id":        workspaceID,
		"deletion_protection": deletionProtectionLabel(*in.Enabled),
	})
}

func deletionProtectionLabel(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

// deleteWorkspace permanently deletes a workspace. This is irreversible and is hidden by
// --read-only as a mutating tool; workspaces with deletion protection enabled are
// rejected by the backend and the error is propagated as-is. This fills the gap left by
// the legacy tool set, which only offered pause/restore and could not delete workspaces via MCP.
func deleteWorkspace(ctx context.Context, p *policy, in workspaceTarget) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if _, err := client.DeleteWorkspace(ctx, workspaceID); err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "workspace_id": workspaceID, "action": "delete"})
}
