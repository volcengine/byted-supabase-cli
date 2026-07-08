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

type UpdateParams struct {
	WorkspaceID           string
	BranchID              string
	ComputeID             string
	ComputeName           string
	AutoScalingLimitMinCU float64
	AutoScalingLimitMaxCU float64
	ModifySpec            bool
}

func Update(ctx context.Context, params UpdateParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, client, err := resolveWorkspace(ctx, cfgSet, params.WorkspaceID)
	if err != nil {
		return err
	}
	if branchID := strings.TrimSpace(params.BranchID); branchID != "" {
		found, err := computeExistsInBranch(ctx, client, workspace.WorkspaceID, branchID, params.ComputeID)
		if err != nil {
			return err
		}
		if !found {
			return errors.Errorf("compute %s was not found in branch %s", params.ComputeID, branchID)
		}
	}
	current, err := client.DescribeComputeDetail(ctx, workspace.WorkspaceID, params.ComputeID)
	if err != nil {
		return err
	}
	if current.ComputeID == "" {
		return errors.Errorf("volcengine compute %s not found", params.ComputeID)
	}
	title := fmt.Sprintf("Do you want to update compute %s in project %s?", utils.Aqua(params.ComputeID), utils.Aqua(workspace.WorkspaceID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	writeClient := volcengine.NewWriteClient(cfg)
	updated := current
	if params.ComputeName != "" {
		if err := writeClient.ModifyComputeName(ctx, volcengine.ModifyComputeNameParams{
			WorkspaceID: workspace.WorkspaceID,
			ComputeID:   params.ComputeID,
			ComputeName: params.ComputeName,
		}); err != nil {
			return err
		}
		updated.ComputeName = params.ComputeName
	}
	if params.ModifySpec {
		specUpdated, err := writeClient.ModifyComputeSpec(ctx, volcengine.ModifyComputeSpecParams{
			WorkspaceID:           workspace.WorkspaceID,
			ComputeID:             params.ComputeID,
			AutoScalingLimitMinCU: params.AutoScalingLimitMinCU,
			AutoScalingLimitMaxCU: params.AutoScalingLimitMaxCU,
		})
		if err != nil {
			return err
		}
		if specUpdated.ComputeID != "" {
			updated = specUpdated
			if params.ComputeName != "" {
				updated.ComputeName = params.ComputeName
			}
		} else {
			updated.AutoScalingLimitMinCU = params.AutoScalingLimitMinCU
			updated.AutoScalingLimitMaxCU = params.AutoScalingLimitMaxCU
		}
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		fmt.Printf("Updated compute %s:\n", utils.Aqua(params.ComputeID))
		if params.ComputeName != "" {
			fmt.Printf("  ComputeName: %s\n", updated.ComputeName)
		}
		if params.ModifySpec {
			fmt.Printf("  AutoScalingLimitMinCU: %v\n", updated.AutoScalingLimitMinCU)
			fmt.Printf("  AutoScalingLimitMaxCU: %v\n", updated.AutoScalingLimitMaxCU)
		}
		return nil
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Compute volcengine.Compute `toml:"compute"`
		}{
			Compute: updated,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, updated)
}

func computeExistsInBranch(ctx context.Context, client *volcengine.Client, workspaceID, branchID, computeID string) (bool, error) {
	computeID = strings.TrimSpace(computeID)
	result, err := client.DescribeBranchComputes(ctx, workspaceID, branchID, "")
	if err != nil {
		return false, err
	}
	for _, compute := range result.Computes {
		if compute.ComputeID == computeID {
			return true, nil
		}
	}
	return false, nil
}

func resolveWorkspace(ctx context.Context, cfgSet volcengine.ConfigSet, workspaceID string) (volcengine.Workspace, volcengine.Config, *volcengine.Client, error) {
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
			return volcengine.Workspace{}, volcengine.Config{}, nil, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		return workspace, cfg, client, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, nil, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, volcengine.Config{}, nil, errors.Errorf("volcengine project %s not found", workspaceID)
}
