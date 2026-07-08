// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package list

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type RunVolcengineParams struct {
	WorkspaceID string
	BranchID    string
}

func RunVolcengine(ctx context.Context, params RunVolcengineParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, params.WorkspaceID)
	if err != nil {
		return err
	}
	client := volcengine.NewClient(cfg)
	branchID := strings.TrimSpace(params.BranchID)
	if branchID == "" {
		defaultBranch, err := client.DescribeDefaultBranch(ctx, workspace.WorkspaceID)
		if err != nil {
			return err
		}
		branchID = defaultBranch.Branch.BranchID
		if branchID == "" {
			return errors.Errorf("failed to resolve default branch for workspace %s", workspace.WorkspaceID)
		}
	}
	result, err := client.DescribeWorkspaceEndpoints(ctx, volcengine.DescribeWorkspaceEndpointsParams{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
	})
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toMarkdown(result.Endpoints))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			WorkspaceID string                `toml:"workspace_id"`
			BranchID    string                `toml:"branch_id"`
			Endpoints   []volcengine.Endpoint `toml:"endpoints"`
		}{
			WorkspaceID: result.WorkspaceID,
			BranchID:    result.BranchID,
			Endpoints:   result.Endpoints,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func toMarkdown(endpoints []volcengine.Endpoint) string {
	var table strings.Builder
	table.WriteString(`|ENDPOINT ID|ENDPOINT NAME|ENDPOINT TYPE|ADDRESS TYPE|DOMAIN|PORT|IP|IPV6|
|-|-|-|-|-|-|-|-|
`)
	for _, endpoint := range endpoints {
		if len(endpoint.Addresses) == 0 {
			fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|-|-|-|-|-|\n",
				endpoint.EndpointID,
				escape(endpoint.EndpointName),
				endpoint.EndpointType)
			continue
		}
		for _, address := range endpoint.Addresses {
			fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%d`|`%s`|`%s`|\n",
				endpoint.EndpointID,
				escape(endpoint.EndpointName),
				endpoint.EndpointType,
				address.AddressType,
				escape(address.AddressDomain),
				address.AddressPort,
				address.IPAddress,
				address.IPv6Address)
		}
	}
	return table.String()
}

func escape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func findSupabaseWorkspace(ctx context.Context, cfgSet volcengine.ConfigSet, workspaceID string) (volcengine.Workspace, volcengine.Config, error) {
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		result, err := volcengine.NewClient(cfg).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := result.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		return workspace, cfg, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("volcengine project %s not found", workspaceID)
}
