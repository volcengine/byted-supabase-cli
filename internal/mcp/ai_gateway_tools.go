// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/aigateway"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func aiGatewayTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "ai_gateway_status",
			title:       "Check AI Gateway status",
			feature:     featureAIGateway,
			description: "Check whether the AI Gateway for a workspace branch is reachable and return its latency.",
		}, aiGatewayStatus),
		defineTool(meta{
			name:        "ai_gateway_models",
			title:       "List AI Gateway models",
			feature:     featureAIGateway,
			description: "List the models available through the AI Gateway for a workspace branch.",
		}, aiGatewayModels),
		defineTool(meta{
			name:        "ai_gateway_chat",
			title:       "Call AI Gateway chat",
			feature:     featureAIGateway,
			mutating:    true,
			description: "Call the OpenAI-compatible AI Gateway chat completions API. Requires a model and either a message or messages.",
		}, aiGatewayChat),
		defineTool(meta{
			name:        "ai_gateway_get_quota",
			title:       "Get AI Gateway quota",
			feature:     featureAIGateway,
			description: "Get the default daily token and image quota for a workspace branch.",
		}, aiGatewayGetQuota),
		defineTool(meta{
			name:        "ai_gateway_set_quota",
			title:       "Set AI Gateway quota",
			feature:     featureAIGateway,
			mutating:    true,
			description: "Set the default daily AI Gateway token and/or image quota. Use -1 for unlimited; at least one limit is required.",
		}, aiGatewaySetQuota),
		defineTool(meta{
			name:        "ai_gateway_list_users",
			title:       "List AI Gateway users",
			feature:     featureAIGateway,
			description: "List AI Gateway users, their custom quotas, and token/image usage for a workspace branch.",
		}, aiGatewayListUsers),
		defineTool(meta{
			name:        "ai_gateway_get_user",
			title:       "Get AI Gateway user quota",
			feature:     featureAIGateway,
			description: "Get one AI Gateway user's quota and usage by user ID.",
		}, aiGatewayGetUser),
		defineTool(meta{
			name:        "ai_gateway_set_user_quota",
			title:       "Set AI Gateway user quota",
			feature:     featureAIGateway,
			mutating:    true,
			description: "Set or reset one AI Gateway user's custom quota. Use reset=true to restore the default quota.",
		}, aiGatewaySetUserQuota),
		defineTool(meta{
			name:        "ai_gateway_usage",
			title:       "Get AI Gateway usage",
			feature:     featureAIGateway,
			description: "Aggregate today's and total AI Gateway token and image usage for all users on a workspace branch.",
		}, aiGatewayUsage),
	}
}

type aiGatewayTarget struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

type aiGatewayCallInput struct {
	aiGatewayTarget
	Model       string          `json:"model" jsonschema:"model ID returned by ai_gateway_models"`
	Message     string          `json:"message,omitempty" jsonschema:"user message; required when messages is omitted"`
	System      string          `json:"system,omitempty" jsonschema:"optional system message"`
	Messages    json.RawMessage `json:"messages,omitempty" jsonschema:"optional OpenAI-compatible messages JSON array"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty" jsonschema:"return streamed text instead of a JSON completion"`
}

func aiGatewayAccess(ctx context.Context, p *policy, target aiGatewayTarget) (volcengine.PgMetaAccess, string, error) {
	client, workspaceID, err := p.client(ctx, target.WorkspaceID)
	if err != nil {
		return volcengine.PgMetaAccess{}, "", err
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, strings.TrimSpace(target.BranchID))
	if err != nil {
		return volcengine.PgMetaAccess{}, "", err
	}
	return access, workspaceID, nil
}

func aiGatewayClient(ctx context.Context, p *policy, target aiGatewayTarget) (aigateway.Client, string, string, error) {
	access, workspaceID, err := aiGatewayAccess(ctx, p, target)
	if err != nil {
		return aigateway.Client{}, "", "", err
	}
	return aigateway.Client{BaseURL: access.BaseURL, Token: access.ServiceRoleKey}, workspaceID, access.BranchID, nil
}

func aiGatewayStatus(ctx context.Context, p *policy, in aiGatewayTarget) (string, error) {
	client, workspaceID, branchID, err := aiGatewayClient(ctx, p, in)
	if err != nil {
		return "", err
	}
	latency, checkErr := client.Check(ctx)
	result := map[string]any{
		"workspace_id": workspaceID,
		"branch_id":    branchID,
		"gateway_url":  strings.TrimRight(client.BaseURL, "/"),
	}
	if checkErr != nil {
		result["status"] = "unavailable"
		result["error"] = checkErr.Error()
	} else {
		result["status"] = "ready"
		result["latency_ms"] = latency.Milliseconds()
	}
	return toJSON(result)
}

func aiGatewayModels(ctx context.Context, p *policy, in aiGatewayTarget) (string, error) {
	client, workspaceID, branchID, err := aiGatewayClient(ctx, p, in)
	if err != nil {
		return "", err
	}
	result, err := client.Models(ctx)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"workspace_id": workspaceID, "branch_id": branchID, "models": result.Data})
}

func aiGatewayChat(ctx context.Context, p *policy, in aiGatewayCallInput) (string, error) {
	if strings.TrimSpace(in.Model) == "" {
		return "", errors.New("model is required")
	}
	client, workspaceID, branchID, err := aiGatewayClient(ctx, p, in.aiGatewayTarget)
	if err != nil {
		return "", err
	}
	messages := in.Messages
	if len(messages) == 0 {
		messages, err = aigateway.NewMessages(in.System, in.Message)
		if err != nil {
			return "", err
		}
	}
	var output bytes.Buffer
	err = client.Chat(ctx, aigateway.ChatRequest{
		Model: in.Model, Messages: messages, Temperature: in.Temperature,
		MaxTokens: in.MaxTokens, TopP: in.TopP, Stream: in.Stream,
	}, &output)
	if err != nil {
		return "", err
	}
	if in.Stream {
		return output.String(), nil
	}
	var response json.RawMessage = output.Bytes()
	if !json.Valid(response) {
		return "", errors.New("AI Gateway returned invalid JSON")
	}
	return toJSON(map[string]any{"workspace_id": workspaceID, "branch_id": branchID, "response": response})
}

type aiGatewayQuotaInput struct {
	aiGatewayTarget
	DailyTokenLimit *int64 `json:"daily_token_limit,omitempty" jsonschema:"daily token limit; use -1 for unlimited"`
	DailyImageLimit *int64 `json:"daily_image_limit,omitempty" jsonschema:"daily image generation limit; use -1 for unlimited"`
}

func aiGatewayManagementClient(ctx context.Context, p *policy, target aiGatewayTarget) (*volcengine.Client, string, string, error) {
	client, workspaceID, err := p.client(ctx, target.WorkspaceID)
	if err != nil {
		return nil, "", "", err
	}
	branchID := strings.TrimSpace(target.BranchID)
	if branchID == "" {
		branch, err := client.ResolveDefaultBranch(ctx, workspaceID)
		if err != nil {
			return nil, "", "", err
		}
		branchID = branch.BranchID
	}
	return client, workspaceID, branchID, nil
}

func aiGatewayGetQuota(ctx context.Context, p *policy, in aiGatewayTarget) (string, error) {
	client, workspaceID, branchID, err := aiGatewayManagementClient(ctx, p, in)
	if err != nil {
		return "", err
	}
	quota, err := client.DescribeAIGlobalQuota(ctx, workspaceID, branchID)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"workspace_id": workspaceID, "branch_id": branchID, "daily_token_limit": quota.DailyTokenLimit, "daily_image_limit": quota.DailyImageLimit})
}

func aiGatewaySetQuota(ctx context.Context, p *policy, in aiGatewayQuotaInput) (string, error) {
	if in.DailyTokenLimit == nil && in.DailyImageLimit == nil {
		return "", errors.New("at least one of daily_token_limit or daily_image_limit is required")
	}
	client, workspaceID, branchID, err := aiGatewayManagementClient(ctx, p, in.aiGatewayTarget)
	if err != nil {
		return "", err
	}
	if err := client.ModifyAIGlobalQuotaWithLimits(ctx, workspaceID, branchID, in.DailyTokenLimit, in.DailyImageLimit); err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "workspace_id": workspaceID, "branch_id": branchID})
}

type aiGatewayUsersInput struct {
	aiGatewayTarget
	Limit  int `json:"limit,omitempty" jsonschema:"maximum users to return; defaults to 10"`
	Offset int `json:"offset,omitempty" jsonschema:"pagination offset; defaults to 0"`
}

func aiGatewayListUsers(ctx context.Context, p *policy, in aiGatewayUsersInput) (string, error) {
	client, workspaceID, branchID, err := aiGatewayManagementClient(ctx, p, in.aiGatewayTarget)
	if err != nil {
		return "", err
	}
	result, err := client.DescribeAIUserSummaries(ctx, volcengine.DescribeAIUserSummariesParams{
		WorkspaceID: workspaceID, BranchID: branchID, Limit: listLimit(in.Limit), Offset: in.Offset,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"workspace_id": workspaceID, "branch_id": branchID, "users": result.Users, "total": result.Total})
}

type aiGatewayUserInput struct {
	aiGatewayTarget
	UserID string `json:"user_id" jsonschema:"AI Gateway user ID"`
}

func aiGatewayGetUser(ctx context.Context, p *policy, in aiGatewayUserInput) (string, error) {
	if strings.TrimSpace(in.UserID) == "" {
		return "", errors.New("user_id is required")
	}
	client, workspaceID, branchID, err := aiGatewayManagementClient(ctx, p, in.aiGatewayTarget)
	if err != nil {
		return "", err
	}
	for offset := 0; ; offset += 100 {
		result, err := client.DescribeAIUserSummaries(ctx, volcengine.DescribeAIUserSummariesParams{
			WorkspaceID: workspaceID, BranchID: branchID, Limit: 100, Offset: offset,
		})
		if err != nil {
			return "", err
		}
		for _, user := range result.Users {
			if user.UserID == strings.TrimSpace(in.UserID) {
				return toJSON(user)
			}
		}
		if len(result.Users) == 0 || offset+len(result.Users) >= result.Total {
			break
		}
	}
	return "", errors.Errorf("AI Gateway user %q not found", in.UserID)
}

type aiGatewaySetUserInput struct {
	aiGatewayTarget
	UserID          string `json:"user_id" jsonschema:"AI Gateway user ID"`
	Reset           bool   `json:"reset,omitempty" jsonschema:"restore the user's default quota"`
	DailyTokenLimit *int64 `json:"daily_token_limit,omitempty" jsonschema:"daily token limit; use -1 for unlimited"`
	DailyImageLimit *int64 `json:"daily_image_limit,omitempty" jsonschema:"daily image generation limit; use -1 for unlimited"`
}

func aiGatewaySetUserQuota(ctx context.Context, p *policy, in aiGatewaySetUserInput) (string, error) {
	if strings.TrimSpace(in.UserID) == "" {
		return "", errors.New("user_id is required")
	}
	if in.Reset && (in.DailyTokenLimit != nil || in.DailyImageLimit != nil) {
		return "", errors.New("reset cannot be combined with quota limits")
	}
	if !in.Reset && in.DailyTokenLimit == nil && in.DailyImageLimit == nil {
		return "", errors.New("at least one quota limit or reset=true is required")
	}
	client, workspaceID, branchID, err := aiGatewayManagementClient(ctx, p, in.aiGatewayTarget)
	if err != nil {
		return "", err
	}
	err = client.ModifyAIUserQuotaWithLimits(ctx, volcengine.ModifyAIUserQuotaParams{
		WorkspaceID: workspaceID, BranchID: branchID, UserID: strings.TrimSpace(in.UserID),
		QuotaCustomized: !in.Reset, DailyTokenLimit: in.DailyTokenLimit, DailyImageLimit: in.DailyImageLimit,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{"success": true, "workspace_id": workspaceID, "branch_id": branchID, "user_id": in.UserID, "reset": in.Reset})
}

func aiGatewayUsage(ctx context.Context, p *policy, in aiGatewayTarget) (string, error) {
	client, workspaceID, branchID, err := aiGatewayManagementClient(ctx, p, in)
	if err != nil {
		return "", err
	}
	usage := volcengine.AIUsage{WorkspaceID: workspaceID, BranchID: branchID}
	for offset := 0; ; offset += 100 {
		result, err := client.DescribeAIUserSummaries(ctx, volcengine.DescribeAIUserSummariesParams{
			WorkspaceID: workspaceID, BranchID: branchID, Limit: 100, Offset: offset,
		})
		if err != nil {
			return "", err
		}
		usage.UserCount = result.Total
		for _, user := range result.Users {
			usage.TodayTokens += user.LastDailyTokens
			usage.TotalTokens += user.TotalTokens
			usage.TodayImages += user.LastDailyImages
			usage.TotalImages += user.TotalImages
		}
		if len(result.Users) == 0 || offset+len(result.Users) >= result.Total {
			break
		}
	}
	return toJSON(usage)
}
