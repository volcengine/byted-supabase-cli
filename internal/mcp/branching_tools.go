// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// branchingTools registers the branching group: preview branch lifecycle management, calling volcengine.Client directly.
func branchingTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "list_branches",
			title:       "List branches",
			feature:     featureBranching,
			description: "List branches in a Supabase workspace.",
		}, listBranches),
		defineTool(meta{
			name:        "get_branch",
			title:       "Get branch",
			feature:     featureBranching,
			description: "Get details of a branch in a Supabase workspace.",
		}, getBranch),
		defineTool(meta{
			name:        "get_default_branch",
			title:       "Get default branch",
			feature:     featureBranching,
			description: "Get the default branch of a Supabase workspace.",
		}, getDefaultBranch),
		defineTool(meta{
			name:        "create_branch",
			title:       "Create branch",
			feature:     featureBranching,
			mutating:    true,
			description: "Create a new branch in a Supabase workspace (defaults to \"develop\").",
		}, createBranch),
		defineTool(meta{
			name:        "delete_branch",
			title:       "Delete branch",
			feature:     featureBranching,
			mutating:    true,
			description: "Delete a branch from a Supabase workspace.",
		}, deleteBranch),
		defineTool(meta{
			name:        "restore_branch",
			title:       "Restore branch",
			feature:     featureBranching,
			mutating:    true,
			description: "Restore a branch to a point in time or from a source branch; a backup branch is created.",
		}, restoreBranch),
	}
}

// branchView is the slim projection returned to the client.
type branchView struct {
	BranchID   string `json:"branch_id"`
	BranchName string `json:"branch_name"`
	Status     string `json:"status"`
	Default    bool   `json:"default"`
	ParentID   string `json:"parent_id,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

func toBranchView(b volcengine.Branch) branchView {
	return branchView{
		BranchID:   b.BranchID,
		BranchName: b.BranchName,
		Status:     b.BranchStatus,
		Default:    b.Default,
		CreatedAt:  b.CreateTime,
		UpdatedAt:  b.UpdateTime,
	}
}

func toBranchDetailView(b volcengine.BranchDetail) branchView {
	return branchView{
		BranchID:   b.BranchID,
		BranchName: b.BranchName,
		Status:     b.BranchStatus,
		Default:    b.Default,
		ParentID:   b.ParentBranch.BranchID,
		CreatedAt:  b.CreateTime,
		UpdatedAt:  b.UpdateTime,
	}
}

type listBranchesInput struct {
	Count       int    `json:"count,omitempty" jsonschema:"maximum number of branches to return per page; defaults to 10"`
	Offset      int    `json:"offset,omitempty" jsonschema:"pagination offset (number of branches to skip); defaults to 0"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func listBranches(ctx context.Context, p *policy, in listBranchesInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBranches(ctx, volcengine.DescribeBranchesParams{
		WorkspaceID: workspaceID,
		Limit:       listLimit(in.Count),
		Offset:      in.Offset,
	})
	if err != nil {
		return "", err
	}
	views := make([]branchView, 0, len(result.Branches))
	for _, b := range result.Branches {
		views = append(views, toBranchView(b))
	}
	return toJSON(map[string]any{
		"branches": views,
		"count":    len(views),
		"total":    result.Total,
	})
}

type getBranchInput struct {
	BranchID    string `json:"branch_id" jsonschema:"id of the branch to retrieve"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func getBranch(ctx context.Context, p *policy, in getBranchInput) (string, error) {
	branchID := strings.TrimSpace(in.BranchID)
	if branchID == "" {
		return "", errors.New("branch_id is required")
	}
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBranchDetail(ctx, workspaceID, branchID)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id":   workspaceID,
		"workspace_name": result.WorkspaceName,
		"branch":         toBranchDetailView(result.Branch),
	})
}

type getDefaultBranchInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func getDefaultBranch(ctx context.Context, p *policy, in getDefaultBranchInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeDefaultBranch(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"branch":       toBranchView(result.Branch),
	})
}

type createBranchInput struct {
	Name        string `json:"name,omitempty" jsonschema:"name for the new branch; defaults to \"develop\""`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func createBranch(ctx context.Context, p *policy, in createBranchInput) (string, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "develop"
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.CreateBranch(ctx, volcengine.CreateBranchParams{
		WorkspaceID: workspaceID,
		Name:        name,
	})
	if err != nil {
		return "", err
	}
	view := toBranchDetailView(result.NormalizedBranch())
	if view.BranchName == "" {
		view.BranchName = name // echo back: the API occasionally omits the name
	}
	return toJSON(map[string]any{
		"success":      true,
		"branch_id":    view.BranchID,
		"branch_name":  view.BranchName,
		"workspace_id": workspaceID,
		"branch":       view,
	})
}

type deleteBranchInput struct {
	BranchID    string `json:"branch_id" jsonschema:"id of the branch to delete"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

func deleteBranch(ctx context.Context, p *policy, in deleteBranchInput) (string, error) {
	branchID := strings.TrimSpace(in.BranchID)
	if branchID == "" {
		return "", errors.New("branch_id is required")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if _, err := client.DeleteBranch(ctx, workspaceID, branchID); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":      true,
		"branch_id":    branchID,
		"workspace_id": workspaceID,
	})
}

type restoreBranchInput struct {
	BranchID       string `json:"branch_id" jsonschema:"id of the branch to restore"`
	SourceBranchID string `json:"source_branch_id,omitempty" jsonschema:"optional source branch id to restore data from"`
	Time           string `json:"time" jsonschema:"required target point-in-time (RFC3339, e.g. 2026-05-20T10:00:00Z) to restore the branch to"`
	WorkspaceID    string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
}

// restoreBranch follows the legacy restore_branch semantics: restore the target branch
// in-place to a point in time or from a source branch, returning the backup branch id
// created from the pre-restore state.
func restoreBranch(ctx context.Context, p *policy, in restoreBranchInput) (string, error) {
	branchID := strings.TrimSpace(in.BranchID)
	if branchID == "" {
		return "", errors.New("branch_id is required")
	}
	// The backend requires time (supplying only source returns InvalidParameter), so
	// validate early to give a clear error — matching CLI's ensureVolcengineRestoreTime
	// behaviour and avoiding a needless round-trip to the backend.
	restoreTime := strings.TrimSpace(in.Time)
	if restoreTime == "" {
		return "", errors.New("time is required (RFC3339, e.g. 2026-05-20T10:00:00Z)")
	}
	if _, err := time.Parse(time.RFC3339, restoreTime); err != nil {
		return "", errors.Errorf("invalid time %q, expected RFC3339 format like 2026-05-20T10:00:00Z", restoreTime)
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.BranchRestore(ctx, volcengine.BranchRestoreParams{
		WorkspaceID:    workspaceID,
		BranchID:       branchID,
		Time:           restoreTime,
		SourceBranchID: strings.TrimSpace(in.SourceBranchID),
	})
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"success":      true,
		"branch_id":    branchID,
		"workspace_id": workspaceID,
	}
	if result.BackupBranchID != "" {
		payload["backup_branch_id"] = result.BackupBranchID
	}
	return toJSON(payload)
}
