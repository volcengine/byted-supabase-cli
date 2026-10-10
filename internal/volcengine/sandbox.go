// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"

	"github.com/volcengine/volcengine-go-sdk/volcengine/universal"
)

const sandboxAPIVersion = "2025-10-01"

type SandboxListParams struct {
	WorkspaceID string
	BranchID    string
	SandboxID   string
	Limit       int
	Offset      int
}

type SandboxUserSummaryParams struct {
	WorkspaceID string
	BranchID    string
	UserID      string
	UserName    string
	Email       string
	Limit       int
	Offset      int
}

func (c *Client) DownloadBaasSandboxLogs(ctx context.Context, projectName, workspaceID, branchID, sandboxID string) (map[string]interface{}, error) {
	input := map[string]interface{}{
		"ProjectName": projectName,
		"WorkspaceId": workspaceID,
		"BranchId":    branchID,
		"SandboxId":   sandboxID,
	}
	var output map[string]interface{}
	if err := c.callSandboxUniversal(ctx, "DownloadBaasSandboxLogs", &input, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func (c *Client) OperateBaasSandbox(ctx context.Context, projectName, workspaceID, branchID, sandboxID, operation string) (map[string]interface{}, error) {
	input := map[string]interface{}{
		"ProjectName": projectName,
		"WorkspaceId": workspaceID,
		"BranchId":    branchID,
		"SandboxId":   sandboxID,
		"Operation":   operation,
	}
	var output map[string]interface{}
	if err := c.callSandboxUniversal(ctx, "OperateBaasSandbox", &input, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func (c *Client) DescribeBaasSandboxes(ctx context.Context, params SandboxListParams) (map[string]interface{}, error) {
	var output map[string]interface{}
	input := map[string]interface{}{}
	if params.WorkspaceID != "" {
		input["WorkspaceId"] = params.WorkspaceID
	}
	if params.BranchID != "" {
		input["BranchId"] = params.BranchID
	}
	if params.SandboxID != "" {
		input["SandboxId"] = params.SandboxID
	}
	if params.Limit > 0 {
		input["Limit"] = params.Limit
	}
	if params.Offset > 0 {
		input["Offset"] = params.Offset
	}
	if err := c.callSandboxUniversal(ctx, "DescribeBaasSandboxes", &input, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func (c *Client) DescribeBaasSandboxTemplates(ctx context.Context, workspaceID, branchID string) (map[string]interface{}, error) {
	input := map[string]interface{}{}
	if workspaceID != "" {
		input["WorkspaceId"] = workspaceID
	}
	if branchID != "" {
		input["BranchId"] = branchID
	}
	var output map[string]interface{}
	if err := c.callSandboxUniversal(ctx, "DescribeBaasSandboxTemplates", &input, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func (c *Client) DescribeBaasSandboxUserSummaries(ctx context.Context, params SandboxUserSummaryParams) (map[string]interface{}, error) {
	input := map[string]interface{}{}
	if params.WorkspaceID != "" {
		input["WorkspaceId"] = params.WorkspaceID
	}
	if params.BranchID != "" {
		input["BranchId"] = params.BranchID
	}
	if params.UserID != "" {
		input["UserId"] = params.UserID
	}
	if params.UserName != "" {
		input["UserName"] = params.UserName
	}
	if params.Email != "" {
		input["Email"] = params.Email
	}
	if params.Limit > 0 {
		input["Limit"] = params.Limit
	}
	if params.Offset > 0 {
		input["Offset"] = params.Offset
	}
	var output map[string]interface{}
	if err := c.callSandboxUniversal(ctx, "DescribeBaasSandboxUserSummaries", &input, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func (c *Client) DescribeBaasSandboxGlobalQuota(ctx context.Context, workspaceID, branchID string) (map[string]interface{}, error) {
	input := map[string]interface{}{}
	if workspaceID != "" {
		input["WorkspaceId"] = workspaceID
	}
	if branchID != "" {
		input["BranchId"] = branchID
	}
	var output map[string]interface{}
	if err := c.callSandboxUniversal(ctx, "DescribeBaasSandboxGlobalQuota", &input, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func (c *Client) ModifyBaasSandboxGlobalQuota(ctx context.Context, workspaceID, branchID string, dailySandboxLimit int64) error {
	input := map[string]interface{}{
		"UserDefaultQuota": map[string]interface{}{
			"DailySandboxLimit": dailySandboxLimit,
		},
	}
	if workspaceID != "" {
		input["WorkspaceId"] = workspaceID
	}
	if branchID != "" {
		input["BranchId"] = branchID
	}
	var output map[string]interface{}
	return c.callSandboxUniversal(ctx, "ModifyBaasSandboxGlobalQuota", &input, &output)
}

func (c *Client) callSandboxUniversal(ctx context.Context, action string, input, output interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.uni.DoCallWithType(universal.RequestUniversal{
		ServiceName: aidapServiceName,
		Action:      action,
		Version:     sandboxAPIVersion,
		HttpMethod:  universal.POST,
		ContentType: universal.ApplicationJSON,
	}, input, output)
}
