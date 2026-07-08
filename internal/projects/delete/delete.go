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
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/unlink"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/credentials"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/zalando/go-keyring"
)

func PreRun(ctx context.Context, ref string) error {
	if err := utils.AssertProjectRefIsValid(ref); err != nil {
		return err
	}
	title := fmt.Sprintf("Do you want to delete project %s? This action is irreversible.", utils.Aqua(ref))
	if shouldDelete, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldDelete {
		return errors.New(context.Canceled)
	}
	return nil
}

func Run(ctx context.Context, ref string, fsys afero.Fs) error {
	resp, err := utils.GetSupabase().V1DeleteAProjectWithResponse(ctx, ref)
	if err != nil {
		return errors.Errorf("failed to delete project: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusNotFound:
		return errors.New("Project does not exist:" + utils.Aqua(ref))
	case http.StatusOK:
		break
	default:
		return errors.Errorf("Failed to delete project %s: %s", utils.Aqua(ref), string(resp.Body))
	}

	// Unlink project
	if err := credentials.StoreProvider.Delete(ref); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		fmt.Fprintln(os.Stderr, err)
	}
	if match, err := afero.FileContainsBytes(fsys, utils.ProjectRefPath, []byte(ref)); match {
		if err := unlink.Unlink(ref, fsys); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	} else if err != nil {
		logger := utils.GetDebugLogger()
		fmt.Fprintln(logger, err)
	}

	fmt.Println("Deleted project: " + utils.Aqua(resp.JSON200.Name))
	return nil
}

func RunVolcengine(ctx context.Context, workspaceID string, fsys afero.Fs) error {
	return runVolcengineWorkspaceAction(ctx, workspaceID, fsys, "delete", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if _, err := client.DeleteWorkspace(ctx, workspace.WorkspaceID); err != nil {
			return err
		}
		if linkedWorkspaceID, err := volcengine.LoadLinkedWorkspaceID(fsys); err != nil {
			logger := utils.GetDebugLogger()
			fmt.Fprintln(logger, err)
		} else if linkedWorkspaceID == workspace.WorkspaceID {
			if err := unlink.Unlink(workspace.WorkspaceID, fsys); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		return nil
	})
}

func RunVolcengineStart(ctx context.Context, workspaceID string, fsys afero.Fs) error {
	return runVolcengineWorkspaceAction(ctx, workspaceID, fsys, "start", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		return client.StartWorkspace(ctx, workspace.WorkspaceID)
	})
}

func RunVolcengineStop(ctx context.Context, workspaceID string, fsys afero.Fs) error {
	return runVolcengineWorkspaceAction(ctx, workspaceID, fsys, "stop", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		return client.StopWorkspace(ctx, workspace.WorkspaceID)
	})
}

func runVolcengineWorkspaceAction(ctx context.Context, workspaceID string, fsys afero.Fs, action string, run func(*volcengine.Client, volcengine.Workspace) error) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, workspaceID)
	if err != nil {
		return err
	}
	if err := PreRunVolcengineAction(ctx, action, workspace.WorkspaceID); err != nil {
		return err
	}
	if err := run(volcengine.NewWriteClient(cfg), workspace); err != nil {
		return err
	}
	fmt.Printf("%s project: %s [%s]\n", pastTense(action), utils.Aqua(workspace.WorkspaceName), utils.Aqua(workspace.WorkspaceID))
	return nil
}

func PreRunVolcengine(ctx context.Context, workspaceID string) error {
	return PreRunVolcengineAction(ctx, "delete", workspaceID)
}

func PreRunVolcengineAction(ctx context.Context, action, workspaceID string) error {
	title := fmt.Sprintf("Do you want to %s project %s?", action, utils.Aqua(workspaceID))
	if action == "delete" {
		title = fmt.Sprintf("Do you want to delete project %s? This action is irreversible.", utils.Aqua(workspaceID))
	}
	if shouldDelete, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldDelete {
		return errors.New(context.Canceled)
	}
	return nil
}

func pastTense(action string) string {
	switch action {
	case "start":
		return "Started"
	case "stop":
		return "Stopped"
	case "delete":
		return "Deleted"
	default:
		return action
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
