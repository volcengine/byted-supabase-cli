// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package update

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/branches/list"
	"github.com/volcengine/byted-supabase-cli/internal/branches/pause"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type RunVolcengineParams struct {
	WorkspaceID string
	BranchID    string
	Name        *string
}

func Run(ctx context.Context, branchId string, body api.UpdateBranchBody, fsys afero.Fs) error {
	projectRef, err := pause.GetBranchProjectRef(ctx, branchId)
	if err != nil {
		return err
	}
	resp, err := utils.GetSupabase().V1UpdateABranchConfigWithResponse(ctx, projectRef, body)
	if err != nil {
		return errors.Errorf("failed to update preview branch: %w", err)
	} else if resp.JSON200 == nil {
		if orgSlug, isGated := utils.SuggestUpgradeOnError(ctx, projectRef, "branching_persistent", resp.StatusCode()); isGated {
			telemetry.TrackUpgradeSuggested(ctx, "branching_persistent", orgSlug)
		}
		return errors.Errorf("unexpected update branch status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	fmt.Fprintln(os.Stderr, "Updated preview branch:")
	if utils.OutputFormat.Value == utils.OutputPretty {
		table := list.ToMarkdown([]api.BranchResponse{*resp.JSON200})
		return utils.RenderTable(table)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, *resp.JSON200)
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
	result, err := volcengine.NewWriteClient(cfg).UpdateBranch(ctx, volcengine.UpdateBranchParams{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    params.BranchID,
		Name:        params.Name,
	})
	if err != nil {
		return err
	}
	if result.Branch.BranchID == "" {
		return errors.Errorf("volcengine branch %s not found", params.BranchID)
	}
	fmt.Fprintln(os.Stderr, "Updated preview branch:")
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toVolcengineMarkdown(result.Branch))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Branch volcengine.BranchDetail `toml:"branch"`
		}{
			Branch: result.Branch,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func toVolcengineMarkdown(branch volcengine.BranchDetail) string {
	var table strings.Builder
	table.WriteString(`|WORKSPACE ID|BRANCH ID|NAME|STATUS|DEFAULT|PROTECTED|ARCHIVED|INIT SOURCE|CREATION SOURCE|CREATED AT|UPDATED AT|
|-|-|-|-|-|-|-|-|-|-|-|
`)
	fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%t`|`%t`|`%t`|`%s`|`%s`|`%s`|`%s`|\n",
		branch.WorkspaceID,
		branch.BranchID,
		strings.ReplaceAll(branch.BranchName, "|", "\\|"),
		branch.BranchStatus,
		branch.Default,
		branch.Protected,
		branch.Archived,
		branch.InitSource,
		branch.CreationSource,
		branch.CreateTime,
		branch.UpdateTime)
	return table.String()
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
		if workspace.WorkspaceName == "" {
			workspace.WorkspaceName = workspace.WorkspaceID
		}
		return workspace, cfg, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("volcengine project %s not found", workspaceID)
}
