// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type restartBranchInput struct {
	BranchID    string   `json:"branch_id" jsonschema:"id of the branch whose compute should be restarted"`
	ComputeIDs  []string `json:"compute_ids,omitempty" jsonschema:"optional compute ids; omit to restart all computes on the branch"`
	WorkspaceID string   `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func restartBranch(ctx context.Context, p *policy, in restartBranchInput) (string, error) {
	branchID := strings.TrimSpace(in.BranchID)
	if branchID == "" {
		return "", errors.New("branch_id is required")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	computeIDs := make([]string, 0, len(in.ComputeIDs))
	for _, computeID := range in.ComputeIDs {
		if trimmed := strings.TrimSpace(computeID); trimmed != "" {
			computeIDs = append(computeIDs, trimmed)
		}
	}
	result, err := client.RestartBranch(ctx, volcengine.RestartBranchParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
		ComputeIDs:  computeIDs,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":      true,
		"workspace_id": workspaceID,
		"branch_id":    result.BranchID,
		"compute_ids":  result.ComputeIDs,
		"action":       "restart_compute",
	})
}

// supabaseServiceMinCU is the minimum allowed min CU for the Supabase (BaaS) service.
// Setting the suspend timeout uses ServiceType=Supabase; min CU must be ≥ this value
// or the backend returns InvalidParameter.
const supabaseServiceMinCU = 0.5

// Auto-suspend idle timeout (seconds) value semantics, kept in sync with the CLI's
// --suspend-timeout-seconds validation:
//   - suspendTimeoutDisabled (-1): disable auto-suspend (always-on / never suspend).
//   - 0: unset (platform default, behaves as always-on); usually only seen on instances that
//     have never had the timeout set explicitly.
//   - [suspendTimeoutMin, suspendTimeoutMax]: enable auto-suspend, scale-to-zero after that
//     many idle seconds.
const (
	suspendTimeoutDisabled = -1
	suspendTimeoutMin      = 300
	suspendTimeoutMax      = 604800
)

// validateSuspendTimeoutSeconds validates the auto-suspend idle timeout: it accepts only
// -1 (disable / always-on), 0 (unset), or the [300, 604800] range; invalid values return a
// clear error before any cloud call, avoiding a bare InvalidParameter from the backend.
func validateSuspendTimeoutSeconds(v int) error {
	if v < suspendTimeoutDisabled || (v > 0 && v < suspendTimeoutMin) || v > suspendTimeoutMax {
		return errors.New("invalid suspend_timeout_seconds: expected -1 (disable auto-suspend / always-on), 0, or a value between 300 and 604800")
	}
	return nil
}

// computeTools registers the compute group: view and adjust branch compute units (CU),
// calling volcengine.Client directly and adapting the fork-specific ModifyComputeSpec /
// ModifyComputeName APIs.
func computeTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "get_compute_settings",
			title:       "Get compute settings",
			feature:     featureCompute,
			description: "List the compute (CU) settings for a branch, including auto-scaling min/max CU and status.",
		}, getComputeSettings),
		defineTool(meta{
			name:    "compute_settings",
			title:   "Modify compute settings",
			aliases: []string{"modify_compute_settings", "update_compute_settings"},
			feature: featureCompute,
			// Canonical name stays "modify": this tool's dominant backend call is the
			// workspace-level ModifyComputeSettings API (the CLI surfaces it as `projects
			// compute-settings`, itself described "Modify compute settings"), and it also
			// folds in `computes update`'s rename/resize. update_compute_settings is kept
			// as a hidden alias so the CLI's `computes update` verb still routes here.
			mutating:    true,
			description: "Configure compute autoscaling, the compute auto-sleep timeout, and/or rename a compute. Use this tool with suspend_timeout_seconds only to change the idle-timeout configuration; it calls ModifyComputeSettings and does NOT immediately suspend the workspace. A value >=300 enables compute auto-sleep after that many idle seconds; -1 disables auto-sleep (always-on / never suspend); 0 is the unset default and is effectively always-on. To immediately hibernate the entire workspace, use suspend_workspace (SuspendWorkspace), not this tool and not stop_workspace. To explicitly pause the entire workspace, use stop_workspace (StopWorkspace). Recommend 3600 (60 min) when enabling auto-sleep.",
		}, modifyComputeSettings),
		defineTool(meta{
			name:        "restart_compute",
			title:       "Restart compute",
			aliases:     []string{"restart_branch"},
			feature:     featureCompute,
			mutating:    true,
			description: "Restart branch compute through RestartBranch. This is compute restart, not workspace sleep, workspace pause, or compute auto-sleep. Provide branch_id; optionally provide compute_ids to restart selected computes, otherwise all computes on the branch. Do not use stop_workspace or suspend_workspace for compute restart.",
		}, restartBranch),
	}
}

// computeView is the slim projection returned to the client.
type computeView struct {
	ComputeID             string  `json:"compute_id"`
	ComputeName           string  `json:"compute_name,omitempty"`
	ComputeRole           string  `json:"compute_role,omitempty"`
	ComputeStatus         string  `json:"compute_status,omitempty"`
	ServiceType           string  `json:"service_type,omitempty"`
	BranchID              string  `json:"branch_id,omitempty"`
	WorkspaceID           string  `json:"workspace_id,omitempty"`
	AutoScalingLimitMinCU float64 `json:"auto_scaling_limit_min_cu"`
	AutoScalingLimitMaxCU float64 `json:"auto_scaling_limit_max_cu"`
	EnableAnalytics       string  `json:"enable_analytics,omitempty"`
	LastActiveTime        string  `json:"last_active_time,omitempty"`
	SuspendedTime         string  `json:"suspended_time,omitempty"`
	// SuspendTimeoutSeconds is the workspace-level auto-suspend idle timeout in seconds:
	// >=300 suspends after that many idle seconds; -1 disables auto-suspend (always-on /
	// never suspend); 0 is the unset default (behaves as always-on). Only backfilled when
	// modify_compute_settings sets the suspend timeout; not included in compute-level queries.
	SuspendTimeoutSeconds *int `json:"suspend_timeout_seconds,omitempty"`
}

func toComputeView(c volcengine.Compute) computeView {
	return computeView{
		ComputeID:             c.ComputeID,
		ComputeName:           c.ComputeName,
		ComputeRole:           c.ComputeRole,
		ComputeStatus:         c.ComputeStatus,
		ServiceType:           c.ServiceType,
		BranchID:              c.BranchID,
		WorkspaceID:           c.WorkspaceID,
		AutoScalingLimitMinCU: c.AutoScalingLimitMinCU,
		AutoScalingLimitMaxCU: c.AutoScalingLimitMaxCU,
		EnableAnalytics:       c.EnableAnalytics,
		LastActiveTime:        c.LastActiveTime,
		SuspendedTime:         c.SuspendedTime,
	}
}

type getComputeSettingsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	ServiceType string `json:"service_type,omitempty" jsonschema:"compute service type to filter by; one of Database or Supabase. Defaults to Database when omitted (per the API default), so only Database-service computes are listed unless you pass Supabase. The Supabase value only applies to Supabase-engine workspaces"`
}

func getComputeSettings(ctx context.Context, p *policy, in getComputeSettingsInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	branchID := strings.TrimSpace(in.BranchID)
	if branchID == "" {
		branch, err := client.ResolveDefaultBranch(ctx, workspaceID)
		if err != nil {
			return "", err
		}
		branchID = branch.BranchID
	}
	result, err := client.DescribeBranchComputes(ctx, workspaceID, branchID, in.ServiceType)
	if err != nil {
		return "", err
	}
	views := make([]computeView, 0, len(result.Computes))
	for _, c := range result.Computes {
		views = append(views, toComputeView(c))
	}
	return toJSON(map[string]any{
		"success":      true,
		"workspace_id": workspaceID,
		"branch_id":    branchID,
		"computes":     views,
		"count":        len(views),
	})
}

type modifyComputeSettingsInput struct {
	ComputeID             string   `json:"compute_id,omitempty" jsonschema:"id of the compute to rename or resize; required only with compute_name/min_cu/max_cu. NOT required to set suspend_timeout_seconds (workspace-wide)"`
	MinCU                 *float64 `json:"min_cu,omitempty" jsonschema:"auto-scaling lower bound in compute units (CU); required together with max_cu to resize. Allowed values are discrete: Database-service computes [0.25, 0.5, 1, 2, ... 32]; Supabase-service computes start at 0.5 (no 0.25). The max_cu/min_cu ratio must not exceed 8"`
	MaxCU                 *float64 `json:"max_cu,omitempty" jsonschema:"auto-scaling upper bound in compute units (CU); required together with min_cu to resize. Same discrete allowed values as min_cu (maximum 32); must be greater than min_cu and the max_cu/min_cu ratio must not exceed 8"`
	ComputeName           string   `json:"compute_name,omitempty" jsonschema:"new name for the compute"`
	SuspendTimeoutSeconds *int     `json:"suspend_timeout_seconds,omitempty" jsonschema:"算力自动休眠 idle timeout configuration (not immediate workspace hibernation): >=300 = auto-sleep after idle seconds; -1 = disable auto-sleep (always-on); 0 = unset default (also always-on). This calls ModifyComputeSettings only. Use suspend_workspace for immediate workspace hibernation and stop_workspace for explicit workspace pause. Applies workspace-wide; min_cu/max_cu are backfilled when omitted. Recommend 3600 (60 min) when enabling"`
	WorkspaceID           string   `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func modifyComputeSettings(ctx context.Context, p *policy, in modifyComputeSettingsInput) (string, error) {
	computeID := strings.TrimSpace(in.ComputeID)
	resizing := in.MinCU != nil || in.MaxCU != nil
	renaming := strings.TrimSpace(in.ComputeName) != ""
	suspending := in.SuspendTimeoutSeconds != nil
	if !resizing && !renaming && !suspending {
		return "", errors.New("nothing to modify: provide min_cu and max_cu to resize, compute_name to rename, and/or suspend_timeout_seconds to set the auto-suspend idle timeout")
	}
	if resizing && (in.MinCU == nil || in.MaxCU == nil) {
		return "", errors.New("min_cu and max_cu are required together to resize")
	}
	if suspending {
		if err := validateSuspendTimeoutSeconds(*in.SuspendTimeoutSeconds); err != nil {
			return "", err
		}
	}
	// compute_id is only meaningful for compute-level operations (rename / resize);
	// the suspend timeout is a workspace-level setting handled by the workspace-level
	// ModifyComputeSettings API, which does not take a compute_id. When only the
	// suspend timeout is being set, we don't require compute_id to prevent callers
	// from repeatedly guessing which compute to target.
	if (renaming || resizing) && computeID == "" {
		return "", errors.New("compute_id is required to rename or resize a compute")
	}

	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}

	updated := computeView{ComputeID: computeID, WorkspaceID: workspaceID}
	if renaming {
		name := strings.TrimSpace(in.ComputeName)
		if err := client.ModifyComputeName(ctx, volcengine.ModifyComputeNameParams{
			WorkspaceID: workspaceID,
			ComputeID:   computeID,
			ComputeName: name,
		}); err != nil {
			return "", err
		}
		updated.ComputeName = name
	}

	switch {
	case suspending:
		// Suspend timeout is set via the workspace-level ModifyComputeSettings
		// (ServiceType=Supabase, i.e. the user-facing service). That API also requires
		// min/max CU; when not explicitly provided they are backfilled from the workspace's
		// current values to avoid zeroing out the CU.
		// ⚠️ The Supabase service's min CU floor is 0.5, but the backfill source
		// ComputeSettings reflects the Database service (commonly 0.25). Sending a value
		// below 0.5 causes the backend to return InvalidParameter — clamp min to 0.5.
		minCU, maxCU, err := resolveComputeCU(ctx, client, workspaceID, in.MinCU, in.MaxCU)
		if err != nil {
			return "", err
		}
		if minCU < supabaseServiceMinCU {
			minCU = supabaseServiceMinCU
		}
		if maxCU < minCU {
			maxCU = minCU
		}
		if _, err := client.ModifyComputeSettings(ctx, workspaceID, volcengine.ModifyComputeSettingsParams{
			AutoScalingLimitMinCU: minCU,
			AutoScalingLimitMaxCU: maxCU,
			SuspendTimeoutSeconds: in.SuspendTimeoutSeconds,
			ServiceType:           volcengine.ServiceTypeSupabase,
		}); err != nil {
			return "", err
		}
		// A successful call means the change took effect; echo the values we sent.
		// (The suspend timeout is stored in the BaaS settings; the ComputeSettings in
		// the modify response belongs to the Database service and would return stale
		// values, so we do not rely on it.)
		updated.ServiceType = volcengine.ServiceTypeSupabase
		updated.AutoScalingLimitMinCU = minCU
		updated.AutoScalingLimitMaxCU = maxCU
		suspend := *in.SuspendTimeoutSeconds
		updated.SuspendTimeoutSeconds = &suspend
	case resizing:
		compute, err := client.ModifyComputeSpec(ctx, volcengine.ModifyComputeSpecParams{
			WorkspaceID:           workspaceID,
			ComputeID:             computeID,
			AutoScalingLimitMinCU: *in.MinCU,
			AutoScalingLimitMaxCU: *in.MaxCU,
		})
		if err != nil {
			return "", err
		}
		if compute.ComputeID != "" {
			updated = toComputeView(compute)
			if renaming {
				updated.ComputeName = strings.TrimSpace(in.ComputeName)
			}
		} else {
			updated.AutoScalingLimitMinCU = *in.MinCU
			updated.AutoScalingLimitMaxCU = *in.MaxCU
		}
	}
	return toJSON(map[string]any{
		"success":      true,
		"workspace_id": workspaceID,
		"compute":      updated,
	})
}

// resolveComputeCU returns the min/max CU for the workspace-level ModifyComputeSettings:
// uses explicitly provided values when present, otherwise backfills from the workspace's
// current settings to avoid accidentally zeroing out unspecified CU values.
func resolveComputeCU(ctx context.Context, client *volcengine.Client, workspaceID string, minCU, maxCU *float64) (float64, float64, error) {
	if minCU != nil && maxCU != nil {
		return *minCU, *maxCU, nil
	}
	detail, err := client.DescribeWorkspaceDetail(ctx, workspaceID)
	if err != nil {
		return 0, 0, err
	}
	cur := detail.Workspace.ComputeSettings
	resolvedMin, resolvedMax := cur.AutoScalingLimitMinCU, cur.AutoScalingLimitMaxCU
	if minCU != nil {
		resolvedMin = *minCU
	}
	if maxCU != nil {
		resolvedMax = *maxCU
	}
	return resolvedMin, resolvedMax, nil
}
