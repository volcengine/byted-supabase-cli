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

package create

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/branches/list"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type RunVolcengineParams struct {
	WorkspaceID string
	Name        string
	ParentID    string
	ParentTime  string
}

func Run(ctx context.Context, body api.CreateBranchBody, fsys afero.Fs) error {
	gitBranch := utils.GetGitBranchOrDefault("", fsys)
	if len(body.BranchName) == 0 && len(gitBranch) > 0 {
		title := fmt.Sprintf("Do you want to create a branch named %s?", utils.Aqua(gitBranch))
		if shouldCreate, err := utils.NewConsole().PromptYesNo(ctx, title, true); err != nil {
			return err
		} else if !shouldCreate {
			return errors.New(context.Canceled)
		}
		body.BranchName = gitBranch
		body.GitBranch = &gitBranch
	}

	resp, err := utils.GetSupabase().V1CreateABranchWithResponse(ctx, flags.ProjectRef, body)
	if err != nil {
		return errors.Errorf("failed to create preview branch: %w", err)
	} else if resp.JSON201 == nil {
		if orgSlug, isGated := utils.SuggestUpgradeOnError(ctx, flags.ProjectRef, "branching_limit", resp.StatusCode()); isGated {
			telemetry.TrackUpgradeSuggested(ctx, "branching_limit", orgSlug)
		}
		return errors.Errorf("unexpected create branch status %d: %s", resp.StatusCode(), string(resp.Body))
	}

	fmt.Println("Created preview branch:")
	if utils.OutputFormat.Value == utils.OutputPretty {
		table := list.ToMarkdown([]api.BranchResponse{*resp.JSON201})
		return utils.RenderTable(table)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, *resp.JSON201)
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
	result, err := volcengine.NewWriteClient(cfg).CreateBranch(ctx, volcengine.CreateBranchParams{
		WorkspaceID: workspace.WorkspaceID,
		Name:        params.Name,
		ParentID:    params.ParentID,
		ParentTime:  params.ParentTime,
	})
	if err != nil {
		return err
	}
	result.Branch = result.NormalizedBranch()
	fmt.Fprintln(os.Stderr, "Created preview branch:")
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
	table.WriteString(`|WORKSPACE ID|BRANCH ID|NAME|STATUS|DEFAULT|PROTECTED|ARCHIVED|INIT SOURCE|CREATION SOURCE|PARENT BRANCH ID|START PARENT TIME|CREATED AT|UPDATED AT|
|-|-|-|-|-|-|-|-|-|-|-|-|-|
`)
	fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%t`|`%t`|`%t`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
		branch.WorkspaceID,
		branch.BranchID,
		strings.ReplaceAll(branch.BranchName, "|", "\\|"),
		branch.BranchStatus,
		branch.Default,
		branch.Protected,
		branch.Archived,
		branch.InitSource,
		branch.CreationSource,
		branch.ParentBranch.BranchID,
		branch.StartParentTime,
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
