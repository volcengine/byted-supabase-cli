// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"bytes"
	"context"
	"strings"

	gentypes "github.com/volcengine/byted-supabase-cli/internal/gen/types"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// developmentTools registers the development group: connection info and type generation, all read-only.
// Reuses the fork-specific gen/types.RunVolcengine and volcengine.Client.
func developmentTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "generate_typescript_types",
			title:       "Generate TypeScript types",
			feature:     featureDevelopment,
			description: "Generate TypeScript types for the database schema. Returns TypeScript source.",
		}, generateTypescriptTypes),
		defineTool(meta{
			name:        "get_workspace_url",
			title:       "Get workspace URL",
			feature:     featureDevelopment,
			description: "Get the Supabase base URL (the REST/Auth/Storage API endpoint, i.e. the documented SUPABASE_BASE_URL) for a workspace branch.",
		}, getWorkspaceURL),
		defineTool(meta{
			name:        "get_publishable_keys",
			title:       "Get publishable keys",
			feature:     featureDevelopment,
			description: "Get the API keys for a workspace branch. The platform issues two keys: the anon key (also returned as publishable_key — same value, type Public) and the service_role key (type Service). Values are masked unless reveal is true.",
		}, getPublishableKeys),
		defineTool(meta{
			name:        "list_workspace_operations",
			title:       "List workspace operations",
			feature:     featureDevelopment,
			description: "List operation logs for a Supabase workspace, optionally filtered by branch, compute, action, status, or time range.",
		}, listWorkspaceOperations),
	}
}

type generateTypescriptTypesInput struct {
	Schemas     []string `json:"schemas,omitempty" jsonschema:"schemas to include; defaults to [public]"`
	WorkspaceID string   `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string   `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

// generateTypescriptTypes returns raw TypeScript source (not JSON).
func generateTypescriptTypes(ctx context.Context, p *policy, in generateTypescriptTypesInput) (string, error) {
	schemas, err := normalizeSchemas(in.Schemas)
	if err != nil {
		return "", err
	}
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := gentypes.RunVolcengine(ctx, client, gentypes.VolcengineParams{
		WorkspaceID: workspaceID,
		BranchID:    strings.TrimSpace(in.BranchID),
		Lang:        gentypes.LangTypescript,
		Schemas:     schemas,
	}, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type developmentTarget struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func getWorkspaceURL(ctx context.Context, p *policy, in developmentTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	// Reuses the CLI's branch → pg-meta URL resolution (the same ResolvePgMetaAccess
	// path used by db/functions tools): an empty branch is resolved to the default branch
	// internally, avoiding InvalidParameter from DescribeWorkspaceEndpoints.
	url, branchID, err := client.ResolveBranchBaseURL(ctx, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id":  workspaceID,
		"branch_id":     branchID,
		"workspace_url": url,
		"api_url":       url,
	})
}

type listWorkspaceOperationsInput struct {
	WorkspaceID     string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID        string `json:"branch_id,omitempty" jsonschema:"optional branch id filter"`
	ComputeID       string `json:"compute_id,omitempty" jsonschema:"optional compute id filter"`
	ActionName      string `json:"action_name,omitempty" jsonschema:"optional operation action name filter"`
	Status          string `json:"status,omitempty" jsonschema:"optional operation status filter"`
	CreateTimeStart string `json:"create_time_start,omitempty" jsonschema:"optional operation create time start filter"`
	CreateTimeEnd   string `json:"create_time_end,omitempty" jsonschema:"optional operation create time end filter"`
	Count           int    `json:"count,omitempty" jsonschema:"maximum number of operations to return per page; defaults to 10"`
	Offset          int    `json:"offset,omitempty" jsonschema:"pagination offset; defaults to 0"`
}

func listWorkspaceOperations(ctx context.Context, p *policy, in listWorkspaceOperationsInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeOperations(ctx, volcengine.DescribeOperationsParams{
		WorkspaceID:     workspaceID,
		BranchID:        strings.TrimSpace(in.BranchID),
		ComputeID:       strings.TrimSpace(in.ComputeID),
		ActionName:      strings.TrimSpace(in.ActionName),
		Status:          strings.TrimSpace(in.Status),
		CreateTimeStart: strings.TrimSpace(in.CreateTimeStart),
		CreateTimeEnd:   strings.TrimSpace(in.CreateTimeEnd),
		Limit:           listLimit(in.Count),
		Offset:          in.Offset,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"workspace_id": workspaceID,
		"operations":   result.Operations,
		"count":        len(result.Operations),
		"total":        result.Total,
	})
}

type getPublishableKeysInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
	Reveal      bool   `json:"reveal,omitempty" jsonschema:"return unmasked key values; defaults to false"`
}

// apiKeyView is the slim projection for a single (masked) API key.
type apiKeyView struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

func getPublishableKeys(ctx context.Context, p *policy, in getPublishableKeysInput) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	branchID, err := client.ResolveDefaultBranchID(ctx, workspaceID, in.BranchID)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeAPIKeys(ctx, volcengine.DescribeAPIKeysParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
		Limit:       100,
	})
	if err != nil {
		return "", err
	}
	// Reuse the shared role-based classification (service key matched by both type and name), fixing the old type-only match that missed some keys.
	roles := volcengine.ExtractRoleKeys(result.APIKeys)
	publishableKey, anonKey, serviceRoleKey := roles.Publishable, roles.Anon, roles.ServiceRole
	keys := make([]apiKeyView, 0, len(roles.All))
	for _, key := range roles.All {
		keys = append(keys, apiKeyView{
			Type: key.Type,
			Key:  maskKey(key.Key, in.Reveal),
		})
	}
	return toJSON(map[string]any{
		"workspace_id":     workspaceID,
		"branch_id":        branchID,
		"publishable_key":  maskKey(publishableKey, in.Reveal),
		"anon_key":         maskKey(anonKey, in.Reveal),
		"service_role_key": maskKey(serviceRoleKey, in.Reveal),
		"keys":             keys,
	})
}

// maskKey follows the legacy _mask_key rule: reveal returns the value unchanged; empty stays empty; length ≤12 is fully masked; otherwise keep the first 6 and last 4 characters.
func maskKey(value string, reveal bool) string {
	if reveal || value == "" {
		return value
	}
	if len(value) <= 12 {
		return strings.Repeat("*", len(value))
	}
	return value[:6] + "..." + value[len(value)-4:]
}
