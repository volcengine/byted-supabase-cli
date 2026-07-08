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

package delete

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/branches/pause"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type RunVolcengineParams struct {
	WorkspaceID string
	BranchID    string
}

func Run(ctx context.Context, branchId string, force *bool) error {
	projectRef, err := pause.GetBranchProjectRef(ctx, branchId)
	if err != nil {
		return err
	}
	resp, err := utils.GetSupabase().V1DeleteABranchWithResponse(ctx, projectRef, &api.V1DeleteABranchParams{
		Force: force,
	})
	if err != nil {
		return errors.Errorf("failed to delete preview branch: %w", err)
	} else if resp.StatusCode() != http.StatusOK {
		return errors.Errorf("unexpected delete branch status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	fmt.Fprintln(os.Stderr, "Deleted preview branch:", projectRef)
	return nil
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
	if err := preRunVolcengine(ctx, params.BranchID); err != nil {
		return err
	}
	result, err := volcengine.NewWriteClient(cfg).DeleteBranch(ctx, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	if result.BranchID == "" {
		result.BranchID = params.BranchID
	}
	if result.WorkspaceID == "" {
		result.WorkspaceID = workspace.WorkspaceID
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		fmt.Fprintln(os.Stderr, "Deleted preview branch:", result.BranchID)
		return nil
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			WorkspaceID string `toml:"workspace_id"`
			BranchID    string `toml:"branch_id"`
		}{
			WorkspaceID: result.WorkspaceID,
			BranchID:    result.BranchID,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func preRunVolcengine(ctx context.Context, branchID string) error {
	title := fmt.Sprintf("Do you want to delete branch %s? This action is irreversible.", utils.Aqua(branchID))
	if shouldDelete, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldDelete {
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
