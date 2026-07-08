// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package link

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func RunVolcengine(ctx context.Context, workspaceID string, fsys afero.Fs) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}

	var workspace volcengine.Workspace
	var region string
	var lastErr error
	for _, currentRegion := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(currentRegion)
		detail, err := volcengine.NewClient(cfg).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		if detail.Workspace.WorkspaceID == "" {
			continue
		}
		workspace = detail.Workspace
		region = cfg.Region
		break
	}
	if workspace.WorkspaceID == "" {
		if lastErr != nil {
			return errors.Errorf("failed to retrieve volcengine workspace: %w", lastErr)
		}
		return errors.New("volcengine workspace not found")
	}
	if workspace.EngineType != volcengine.EngineTypeSupabase {
		return errors.Errorf("workspace %s is %s instead of Supabase", workspaceID, workspace.EngineType)
	}
	if workspace.WorkspaceStatus != "Running" {
		fmt.Fprintf(os.Stderr, "%s: Workspace status is %s instead of Running. Some operations might fail.\n", utils.Yellow("WARNING"), workspace.WorkspaceStatus)
	}

	if err := writeVolcengineLinkState(fsys, workspaceID, region); err != nil {
		return err
	}
	return nil
}

func writeVolcengineLinkState(fsys afero.Fs, workspaceID, region string) error {
	entries := map[string]string{
		utils.ProjectRefPath: workspaceID,
		filepath.Join(utils.TempDir, volcengine.LinkedWorkspaceFile): workspaceID,
		filepath.Join(utils.TempDir, volcengine.LinkedRegionFile):    region,
	}
	for path, value := range entries {
		if err := utils.WriteFile(path, []byte(value), fsys); err != nil {
			return err
		}
	}
	return nil
}
