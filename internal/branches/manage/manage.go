// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package manage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type BranchActionParams struct {
	WorkspaceID    string
	BranchID       string
	ComputeIDs     []string
	RestoreTime    string
	SourceBranchID string
	Search         string
	Limit          int
	Offset         int
}

type GetDefaultParams struct {
	WorkspaceID string
}

type UpdateStudioLoginParams struct {
	WorkspaceID string
	BranchID    string
	Username    string
	Password    string
}

type UpdateStudioLoginResult struct {
	WorkspaceID string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
	BranchID    string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	Username    string `json:"Username" toml:"username" yaml:"username"`
}

func RunGetDefault(ctx context.Context, params GetDefaultParams) error {
	return runWorkspaceAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		result, err := client.DescribeDefaultBranch(ctx, workspace.WorkspaceID)
		if err != nil {
			return err
		}
		if result.Branch.BranchID == "" {
			return errors.Errorf("default branch for workspace %s not found", workspace.WorkspaceID)
		}
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			return utils.RenderTable(toBranchMarkdown(branchDetailFromBranch(result.Branch)))
		case utils.OutputToml:
			return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
				Branch volcengine.Branch `toml:"branch"`
			}{
				Branch: result.Branch,
			})
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	})
}

func RunSetDefault(ctx context.Context, params BranchActionParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := confirmBranchAction(ctx, "set branch "+params.BranchID+" as default"); err != nil {
			return err
		}
		result, err := client.SetAsDefaultBranch(ctx, workspace.WorkspaceID, params.BranchID)
		if err != nil {
			return err
		}
		if result.Branch.BranchID == "" {
			return errors.Errorf("volcengine branch %s not found", params.BranchID)
		}
		fmt.Fprintln(os.Stderr, "Set default preview branch:", result.Branch.BranchID)
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			return utils.RenderTable(toBranchMarkdown(result.Branch))
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
	})
}

func RunRestart(ctx context.Context, params BranchActionParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := confirmBranchAction(ctx, "restart branch "+params.BranchID); err != nil {
			return err
		}
		result, err := client.RestartBranch(ctx, volcengine.RestartBranchParams{
			WorkspaceID: workspace.WorkspaceID,
			BranchID:    params.BranchID,
			ComputeIDs:  params.ComputeIDs,
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Restarted preview branch:", result.BranchID)
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			return nil
		case utils.OutputToml:
			return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
				WorkspaceID string   `toml:"workspace_id"`
				BranchID    string   `toml:"branch_id"`
				ComputeIDs  []string `toml:"compute_ids,omitempty"`
			}{
				WorkspaceID: result.WorkspaceID,
				BranchID:    result.BranchID,
				ComputeIDs:  result.ComputeIDs,
			})
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	})
}

func RunUpdateStudioLogin(ctx context.Context, params UpdateStudioLoginParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := confirmBranchAction(ctx, "update Studio login for branch "+params.BranchID); err != nil {
			return err
		}
		if err := client.ResetWorkspaceAccountPassword(ctx, volcengine.ResetWorkspaceAccountPasswordParams{
			WorkspaceID:     workspace.WorkspaceID,
			BranchID:        params.BranchID,
			AccountName:     params.Username,
			AccountPassword: params.Password,
		}); err != nil {
			return err
		}
		result := UpdateStudioLoginResult{
			WorkspaceID: workspace.WorkspaceID,
			BranchID:    params.BranchID,
			Username:    params.Username,
		}
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			fmt.Printf("Updated Studio login for branch %s (username: %s).\n", utils.Aqua(params.BranchID), utils.Aqua(params.Username))
			return nil
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	})
}

func RunRestoreWindow(ctx context.Context, params BranchActionParams) error {
	return runWorkspaceAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		window, err := client.GetRestoreWindow(ctx, workspace.WorkspaceID, params.BranchID)
		if err != nil {
			return err
		}
		if window.BranchID == "" {
			return errors.Errorf("restore window for branch %s not found", params.BranchID)
		}
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			return utils.RenderTable(toRestoreWindowMarkdown([]volcengine.RestoreWindow{window}))
		case utils.OutputToml:
			return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
				RestoreWindow volcengine.RestoreWindow `toml:"restore_window"`
			}{
				RestoreWindow: window,
			})
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, window)
	})
}

func RunRestorable(ctx context.Context, params BranchActionParams) error {
	return runWorkspaceAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		result, err := client.DescribeRestorableBranches(ctx, volcengine.DescribeRestorableBranchesParams{
			WorkspaceID: workspace.WorkspaceID,
			Time:        params.RestoreTime,
			Search:      params.Search,
			Limit:       params.Limit,
			Offset:      params.Offset,
		})
		if err != nil {
			return err
		}
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			return utils.RenderTable(toRestorableBranchesMarkdown(result))
		case utils.OutputToml:
			return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
				Branches       []volcengine.Branch        `toml:"branches"`
				RestoreWindows []volcengine.RestoreWindow `toml:"restore_windows"`
			}{
				Branches:       result.Branches,
				RestoreWindows: result.RestoreWindows,
			})
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	})
}

func RunRestore(ctx context.Context, params BranchActionParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		action := "restore branch " + params.BranchID + " to " + params.RestoreTime
		if params.SourceBranchID != "" {
			action += " from source branch " + params.SourceBranchID
		}
		if err := confirmBranchAction(ctx, action); err != nil {
			return err
		}
		result, err := client.BranchRestore(ctx, volcengine.BranchRestoreParams{
			WorkspaceID:    workspace.WorkspaceID,
			BranchID:       params.BranchID,
			Time:           params.RestoreTime,
			SourceBranchID: params.SourceBranchID,
		})
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Restored preview branch:", result.BranchID)
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			if result.BackupBranchID != "" {
				fmt.Fprintln(os.Stderr, "Backup branch:", result.BackupBranchID)
			}
			return nil
		case utils.OutputToml:
			return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
				WorkspaceID    string `toml:"workspace_id"`
				BranchID       string `toml:"branch_id"`
				SourceBranchID string `toml:"source_branch_id,omitempty"`
				Time           string `toml:"time,omitempty"`
				BackupBranchID string `toml:"backup_branch_id,omitempty"`
			}{
				WorkspaceID:    result.WorkspaceID,
				BranchID:       result.BranchID,
				SourceBranchID: result.SourceBranchID,
				Time:           result.Time,
				BackupBranchID: result.BackupBranchID,
			})
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	})
}

func runWorkspaceAction(ctx context.Context, workspaceID string, run func(*volcengine.Client, volcengine.Workspace) error) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, workspaceID)
	if err != nil {
		return err
	}
	return run(volcengine.NewClient(cfg), workspace)
}

func runWorkspaceWriteAction(ctx context.Context, workspaceID string, run func(*volcengine.Client, volcengine.Workspace) error) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, workspaceID)
	if err != nil {
		return err
	}
	return run(volcengine.NewWriteClient(cfg), workspace)
}

func confirmBranchAction(ctx context.Context, action string) error {
	title := fmt.Sprintf("Do you want to %s?", utils.Aqua(action))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	return nil
}

func toBranchMarkdown(branch volcengine.BranchDetail) string {
	var table strings.Builder
	table.WriteString(`|ID|NAME|STATUS|DEFAULT|PROTECTED|ARCHIVED|INIT SOURCE|CREATED AT|UPDATED AT|
|-|-|-|-|-|-|-|-|-|
`)
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
	return table.String()
}

func toRestoreWindowMarkdown(windows []volcengine.RestoreWindow) string {
	var table strings.Builder
	table.WriteString(`|BRANCH ID|WINDOW SIZE SECONDS|BRANCH CREATED AT|START TIME|END TIME|
|-|-|-|-|-|
`)
	for _, window := range windows {
		fmt.Fprintf(&table, "|`%s`|`%d`|`%s`|`%s`|`%s`|\n",
			window.BranchID,
			window.WindowSizeSeconds,
			window.BranchCreationTime,
			window.StartTime,
			window.EndTime)
	}
	return table.String()
}

func toRestorableBranchesMarkdown(result volcengine.DescribeRestorableBranchesResult) string {
	windows := make(map[string]volcengine.RestoreWindow, len(result.RestoreWindows))
	for _, window := range result.RestoreWindows {
		windows[window.BranchID] = window
	}
	var table strings.Builder
	table.WriteString(`|ID|NAME|STATUS|DEFAULT|START TIME|END TIME|WINDOW SIZE SECONDS|
|-|-|-|-|-|-|-|
`)
	for _, branch := range result.Branches {
		window := windows[branch.BranchID]
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%t`|`%s`|`%s`|`%d`|\n",
			branch.BranchID,
			strings.ReplaceAll(branch.BranchName, "|", "\\|"),
			branch.BranchStatus,
			branch.Default,
			window.StartTime,
			window.EndTime,
			window.WindowSizeSeconds)
	}
	return table.String()
}

func branchDetailFromBranch(branch volcengine.Branch) volcengine.BranchDetail {
	return volcengine.BranchDetail{
		WorkspaceID:  branch.WorkspaceID,
		BranchID:     branch.BranchID,
		BranchName:   branch.BranchName,
		BranchStatus: branch.BranchStatus,
		Default:      branch.Default,
		Protected:    branch.Protected,
		Archived:     branch.Archived,
		InitSource:   branch.InitSource,
		CreateTime:   branch.CreateTime,
		UpdateTime:   branch.UpdateTime,
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
