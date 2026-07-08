// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package describe

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type ListParams struct {
	WorkspaceID string
	BranchID    string
	ServiceType string
}

type GetParams struct {
	WorkspaceID string
	ComputeID   string
}

func RunList(ctx context.Context, params ListParams) error {
	_, result, err := listSupabaseComputes(ctx, params)
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toMarkdown(result.Computes))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Computes []volcengine.Compute `toml:"computes"`
		}{
			Computes: result.Computes,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result.Computes)
}

func RunGet(ctx context.Context, params GetParams) error {
	compute, err := GetCompute(ctx, params)
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toMarkdown([]volcengine.Compute{compute}))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Compute volcengine.Compute `toml:"compute"`
		}{
			Compute: compute,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, compute)
}

func GetCompute(ctx context.Context, params GetParams) (volcengine.Compute, error) {
	workspace, client, err := resolveWorkspace(ctx, params.WorkspaceID)
	if err != nil {
		return volcengine.Compute{}, err
	}
	compute, err := client.DescribeComputeDetail(ctx, workspace.WorkspaceID, strings.TrimSpace(params.ComputeID))
	if err != nil {
		return volcengine.Compute{}, err
	}
	if compute.ComputeID == "" {
		return volcengine.Compute{}, errors.Errorf("volcengine compute %s not found", params.ComputeID)
	}
	return compute, nil
}

func PromptComputeID(ctx context.Context, params ListParams) (string, error) {
	branchID, result, err := listSupabaseComputes(ctx, params)
	if err != nil {
		return "", err
	}
	if len(result.Computes) == 0 {
		if branchID != "" {
			if params.ServiceType != "" {
				return "", errors.Errorf("no Volcengine computes found for branch %s with service type %s", branchID, params.ServiceType)
			}
			return "", errors.Errorf("no Volcengine computes found for branch %s", branchID)
		}
		return "", errors.New("no Volcengine computes found")
	}
	items := make([]utils.PromptItem, len(result.Computes))
	for i, compute := range result.Computes {
		items[i] = utils.PromptItem{
			Summary: compute.ComputeID,
			Details: "name: " + compute.ComputeName + ", branch: " + compute.BranchID + ", role: " + compute.ComputeRole + ", service: " + compute.ServiceType + ", status: " + compute.ComputeStatus,
		}
	}
	choice, err := utils.PromptChoice(ctx, "Which compute do you want to select?", items)
	if err != nil {
		return "", err
	}
	return choice.Summary, nil
}

func listSupabaseComputes(ctx context.Context, params ListParams) (string, volcengine.DescribeComputesResult, error) {
	workspace, client, err := resolveWorkspace(ctx, params.WorkspaceID)
	if err != nil {
		return "", volcengine.DescribeComputesResult{}, err
	}
	branchID, err := client.ResolveDefaultBranchID(ctx, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return "", volcengine.DescribeComputesResult{}, err
	}
	result, err := client.DescribeBranchComputes(ctx, workspace.WorkspaceID, branchID, params.ServiceType)
	if err != nil {
		return branchID, result, err
	}
	return branchID, result, nil
}

func resolveWorkspace(ctx context.Context, workspaceID string) (volcengine.Workspace, *volcengine.Client, error) {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return volcengine.Workspace{}, nil, err
	}
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		client := volcengine.NewClient(cfg)
		result, err := client.DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := result.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Workspace{}, nil, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		return workspace, client, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, nil, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, nil, errors.Errorf("volcengine project %s not found", workspaceID)
}

func toMarkdown(computes []volcengine.Compute) string {
	var table strings.Builder
	table.WriteString(`|COMPUTE ID|NAME|BRANCH ID|STATUS|ROLE|SERVICE TYPE|MIN CU|MAX CU|ANALYTICS|DISABLED|CREATED AT|
|-|-|-|-|-|-|-|-|-|-|-|
`)
	for _, compute := range computes {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%g`|`%g`|`%s`|`%t`|`%s`|\n",
			compute.ComputeID,
			strings.ReplaceAll(compute.ComputeName, "|", "\\|"),
			compute.BranchID,
			compute.ComputeStatus,
			compute.ComputeRole,
			compute.ServiceType,
			compute.AutoScalingLimitMinCU,
			compute.AutoScalingLimitMaxCU,
			compute.EnableAnalytics,
			compute.Disabled,
			compute.CreateTime)
	}
	return table.String()
}
