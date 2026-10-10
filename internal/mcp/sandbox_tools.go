// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func sandboxTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "list_sandbox_templates",
			title:       "List sandbox templates",
			feature:     featureSandbox,
			description: "List available Supabase Sandbox templates for code execution. Common templates include opencode, codex, code-interpreter, and base.",
		}, listSandboxTemplates),
		defineTool(meta{
			name:        "list_sandboxes",
			title:       "List sandboxes",
			feature:     featureSandbox,
			description: "List Supabase Sandboxes. Provide sandbox_id to query one sandbox by ID.",
		}, listSandboxes),
		defineTool(meta{
			name:        "list_sandbox_user_summaries",
			title:       "List sandbox user summaries",
			feature:     featureSandbox,
			description: "List Sandbox user permissions and quota summaries. Filter by user_id, user_name, or email.",
		}, listSandboxUserSummaries),
		defineTool(meta{
			name:        "get_sandbox_global_quota",
			title:       "Get sandbox global quota",
			feature:     featureSandbox,
			description: "Get the default global daily Sandbox quota. The platform default is 100 sandboxes per user per day unless the returned configuration says otherwise.",
		}, getSandboxGlobalQuota),
		defineTool(meta{
			name:        "set_sandbox_global_quota",
			title:       "Set sandbox global quota",
			feature:     featureSandbox,
			mutating:    true,
			description: "Set the default global daily Sandbox quota per user. The value must be non-negative.",
		}, setSandboxGlobalQuota),
	}
}

type sandboxTarget struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; required by Sandbox management APIs"`
}

type listSandboxesInput struct {
	sandboxTarget
	SandboxID string `json:"sandbox_id,omitempty" jsonschema:"optional sandbox ID; when supplied, query that sandbox"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum sandboxes to return; defaults to 10"`
	Offset    int    `json:"offset,omitempty" jsonschema:"pagination offset; defaults to 0"`
}

func listSandboxes(ctx context.Context, p *policy, in listSandboxesInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBaasSandboxes(ctx, volcengine.SandboxListParams{
		WorkspaceID: workspaceID,
		SandboxID:   strings.TrimSpace(in.SandboxID),
		Limit:       listLimit(in.Limit),
		Offset:      in.Offset,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"result":       result,
	})
}

type listSandboxTemplatesInput struct {
	sandboxTarget
}

func listSandboxTemplates(ctx context.Context, p *policy, in listSandboxTemplatesInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBaasSandboxTemplates(ctx, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"templates":    result,
		"known_templates": []string{
			"opencode",
			"codex",
			"code-interpreter",
			"base",
		},
	})
}

type listSandboxUserSummariesInput struct {
	sandboxTarget
	UserID   string `json:"user_id,omitempty" jsonschema:"optional user ID filter"`
	UserName string `json:"user_name,omitempty" jsonschema:"optional user name filter"`
	Email    string `json:"email,omitempty" jsonschema:"optional email filter"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum users to return; defaults to 10"`
	Offset   int    `json:"offset,omitempty" jsonschema:"pagination offset; defaults to 0"`
}

func listSandboxUserSummaries(ctx context.Context, p *policy, in listSandboxUserSummariesInput) (string, error) {
	if in.Limit < 0 || in.Offset < 0 {
		return "", fmt.Errorf("limit and offset must be non-negative")
	}
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBaasSandboxUserSummaries(ctx, volcengine.SandboxUserSummaryParams{
		WorkspaceID: workspaceID,
		BranchID:    strings.TrimSpace(in.BranchID),
		UserID:      strings.TrimSpace(in.UserID),
		UserName:    strings.TrimSpace(in.UserName),
		Email:       strings.TrimSpace(in.Email),
		Limit:       listLimit(in.Limit),
		Offset:      in.Offset,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"result":       result,
	})
}

func getSandboxGlobalQuota(ctx context.Context, p *policy, in sandboxTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeBaasSandboxGlobalQuota(ctx, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id":            workspaceID,
		"default_daily_sandboxes": 100,
		"result":                  result,
	})
}

type setSandboxGlobalQuotaInput struct {
	sandboxTarget
	DailySandboxLimit *int64 `json:"daily_sandbox_limit" jsonschema:"default daily Sandbox quota per user per day; must be non-negative"`
}

func setSandboxGlobalQuota(ctx context.Context, p *policy, in setSandboxGlobalQuotaInput) (string, error) {
	if in.DailySandboxLimit == nil {
		return "", fmt.Errorf("daily_sandbox_limit is required")
	}
	if *in.DailySandboxLimit < 0 {
		return "", fmt.Errorf("daily_sandbox_limit must be greater than or equal to 0")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if err := client.ModifyBaasSandboxGlobalQuota(ctx, workspaceID, strings.TrimSpace(in.BranchID), *in.DailySandboxLimit); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":             true,
		"workspace_id":        workspaceID,
		"daily_sandbox_limit": *in.DailySandboxLimit,
	})
}
