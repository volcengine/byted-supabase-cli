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

package list

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type RunVolcengineParams struct {
	WorkspaceID string
	Search      string
	ParentID    string
	Limit       int
	Offset      int
}

func Run(ctx context.Context, fsys afero.Fs) error {
	branches, err := ListBranch(ctx, flags.ProjectRef)
	if err != nil {
		return err
	}

	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := ToMarkdown(branches)
		return utils.RenderTable(table)
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Branches []api.BranchResponse `toml:"branches"`
		}{
			Branches: branches,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}

	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, branches)
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
	var result volcengine.DescribeBranchesResult
	if params.ParentID != "" {
		query := volcengine.DescribeChildBranchesParams{
			WorkspaceID: workspace.WorkspaceID,
			ParentID:    params.ParentID,
			Limit:       params.Limit,
			Offset:      params.Offset,
		}
		if params.Limit > 0 {
			result, err = volcengine.NewClient(cfg).DescribeChildBranches(ctx, query)
		} else {
			result, err = volcengine.NewClient(cfg).DescribeAllChildBranches(ctx, query)
		}
	} else {
		query := volcengine.DescribeBranchesParams{
			WorkspaceID: workspace.WorkspaceID,
			Search:      params.Search,
			Limit:       params.Limit,
			Offset:      params.Offset,
		}
		if params.Limit > 0 {
			result, err = volcengine.NewClient(cfg).DescribeBranches(ctx, query)
		} else {
			result, err = volcengine.NewClient(cfg).DescribeAllBranches(ctx, query)
		}
	}
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toVolcengineMarkdown(result.Branches))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Branches []volcengine.Branch `toml:"branches"`
		}{
			Branches: result.Branches,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result.Branches)
}

func ToMarkdown(branches []api.BranchResponse) string {
	var table strings.Builder
	table.WriteString(`|ID|NAME|DEFAULT|GIT BRANCH|WITH DATA|STATUS|CREATED AT (UTC)|UPDATED AT (UTC)|
|-|-|-|-|-|-|-|-|
`)
	for _, branch := range branches {
		gitBranch := " "
		if branch.GitBranch != nil {
			gitBranch = *branch.GitBranch
		}
		fmt.Fprintf(&table, "|`%s`|`%s`|`%t`|`%s`|`%t`|`%s`|`%s`|`%s`|\n",
			branch.ProjectRef,
			strings.ReplaceAll(branch.Name, "|", "\\|"),
			branch.IsDefault,
			strings.ReplaceAll(gitBranch, "|", "\\|"),
			branch.WithData,
			branch.Status,
			utils.FormatTime(branch.CreatedAt),
			utils.FormatTime(branch.UpdatedAt))
	}
	return table.String()
}

func toVolcengineMarkdown(branches []volcengine.Branch) string {
	var table strings.Builder
	table.WriteString(`|ID|NAME|STATUS|DEFAULT|PROTECTED|ARCHIVED|INIT SOURCE|CREATED AT|UPDATED AT|
|-|-|-|-|-|-|-|-|-|
`)
	for _, branch := range branches {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%t`|`%t`|`%t`|`%s`|`%s`|`%s`|\n",
			branch.BranchID,
			strings.ReplaceAll(branch.BranchName, "|", "\\|"),
			branch.BranchStatus,
			branch.Default,
			branch.Protected,
			branch.Archived,
			branch.InitSource,
			branch.CreateTime,
			branch.UpdateTime)
	}
	return table.String()
}

type BranchFilter func(api.BranchResponse) bool

func ListBranch(ctx context.Context, ref string, filter ...BranchFilter) ([]api.BranchResponse, error) {
	resp, err := utils.GetSupabase().V1ListAllBranchesWithResponse(ctx, ref)
	if err != nil {
		return nil, errors.Errorf("failed to list branch: %w", err)
	} else if resp.JSON200 == nil {
		return nil, errors.Errorf("unexpected list branch status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	var result []api.BranchResponse
OUTER:
	for _, branch := range *resp.JSON200 {
		for _, keep := range filter {
			if !keep(branch) {
				continue OUTER
			}
		}
		result = append(result, branch)
	}
	return result, nil
}

func FilterByName(branchName string) BranchFilter {
	return func(br api.BranchResponse) bool {
		return br.Name == branchName
	}
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
