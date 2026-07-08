// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package manage

import (
	"context"
	"fmt"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type RenameParams struct {
	WorkspaceID   string
	WorkspaceName string
}

type DeletionProtectionParams struct {
	WorkspaceID string
	Enabled     bool
}

type TagsParams struct {
	WorkspaceID string
	Tags        []volcengine.WorkspaceTag
	TagKeys     []string
}

type ComputeSettingsParams struct {
	WorkspaceID           string
	AutoScalingLimitMinCU float64
	AutoScalingLimitMaxCU float64
	SuspendTimeoutSeconds *int
	ServiceType           string
}

type WorkspaceSettingsParams struct {
	WorkspaceID           string
	HistoryRetentionHours int
}

func RenameWorkspace(ctx context.Context, params RenameParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.ModifyWorkspaceName(ctx, workspace.WorkspaceID, params.WorkspaceName); err != nil {
			return err
		}
		fmt.Printf("Renamed project: %s [%s] -> %s [%s]\n", utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID), utils.Aqua(params.WorkspaceName), utils.Aqua(workspace.WorkspaceID))
		return nil
	})
}

func ModifyDeletionProtection(ctx context.Context, params DeletionProtectionParams) error {
	status := "Disabled"
	if params.Enabled {
		status = "Enabled"
	}
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.ModifyWorkspaceDeletionProtectionPolicy(ctx, workspace.WorkspaceID, params.Enabled); err != nil {
			return err
		}
		fmt.Printf("Updated deletion protection for project %s [%s]: %s\n", utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID), utils.Aqua(status))
		return nil
	})
}

func CreateTags(ctx context.Context, params TagsParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.AddTagsToWorkspaces(ctx, []string{workspace.WorkspaceID}, params.Tags); err != nil {
			return err
		}
		fmt.Printf("Created tags for project: %s [%s]\n", utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID))
		return nil
	})
}

func DeleteTags(ctx context.Context, params TagsParams) error {
	return runWorkspaceWriteAction(ctx, params.WorkspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.RemoveTagsFromWorkspaces(ctx, []string{workspace.WorkspaceID}, params.TagKeys); err != nil {
			return err
		}
		fmt.Printf("Deleted tags from project: %s [%s]\n", utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID))
		return nil
	})
}

func ModifyComputeSettings(ctx context.Context, params ComputeSettingsParams) error {
	return runConfirmedWorkspaceAction(ctx, params.WorkspaceID, "modify compute settings for", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		updated, err := client.ModifyComputeSettings(ctx, workspace.WorkspaceID, volcengine.ModifyComputeSettingsParams{
			AutoScalingLimitMinCU: params.AutoScalingLimitMinCU,
			AutoScalingLimitMaxCU: params.AutoScalingLimitMaxCU,
			SuspendTimeoutSeconds: params.SuspendTimeoutSeconds,
			ServiceType:           params.ServiceType,
		})
		if err != nil {
			return err
		}
		if updated.WorkspaceID == "" {
			updated = workspace
			updated.ComputeSettings.AutoScalingLimitMinCU = params.AutoScalingLimitMinCU
			updated.ComputeSettings.AutoScalingLimitMaxCU = params.AutoScalingLimitMaxCU
			if params.SuspendTimeoutSeconds != nil {
				updated.ComputeSettings.SuspendTimeoutSeconds = *params.SuspendTimeoutSeconds
			}
		}
		fmt.Printf("Updated compute settings for project %s [%s]:\n", utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID))
		fmt.Printf("  AutoScalingLimitMinCU: %v\n", updated.ComputeSettings.AutoScalingLimitMinCU)
		fmt.Printf("  AutoScalingLimitMaxCU: %v\n", updated.ComputeSettings.AutoScalingLimitMaxCU)
		fmt.Printf("  SuspendTimeoutSeconds: %d\n", updated.ComputeSettings.SuspendTimeoutSeconds)
		return nil
	})
}

func ModifyWorkspaceSettings(ctx context.Context, params WorkspaceSettingsParams) error {
	return runConfirmedWorkspaceAction(ctx, params.WorkspaceID, "modify workspace settings for", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		updated, err := client.ModifyWorkspaceSettings(ctx, workspace.WorkspaceID, volcengine.ModifyWorkspaceSettingsParams{
			HistoryRetentionHours: params.HistoryRetentionHours,
		})
		if err != nil {
			return err
		}
		if updated.WorkspaceID == "" {
			updated = workspace
			updated.WorkspaceSetting.HistoryRetentionHours = params.HistoryRetentionHours
		}
		fmt.Printf("Updated workspace settings for project %s [%s]:\n", utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID))
		fmt.Printf("  HistoryRetentionHours: %d\n", updated.WorkspaceSetting.HistoryRetentionHours)
		return nil
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

func runConfirmedWorkspaceAction(ctx context.Context, workspaceID, action string, run func(*volcengine.Client, volcengine.Workspace) error) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, workspaceID)
	if err != nil {
		return err
	}
	if err := confirmWorkspaceAction(ctx, action, workspace.WorkspaceID); err != nil {
		return err
	}
	return run(volcengine.NewWriteClient(cfg), workspace)
}

func confirmWorkspaceAction(ctx context.Context, action, workspaceID string) error {
	title := fmt.Sprintf("Do you want to %s project %s?", action, utils.Aqua(workspaceID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	return nil
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
