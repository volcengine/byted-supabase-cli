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

const defaultACLName = "acl-default"

type Params struct {
	WorkspaceID string
	IPList      []string
	ModifyMode  string
}

func Get(ctx context.Context, workspaceID string) error {
	return runWorkspaceAction(ctx, workspaceID, func(client *volcengine.Client, workspace volcengine.Workspace) error {
		result, err := client.DescribeAccessControlList(ctx, workspace.WorkspaceID, defaultACLName)
		if err != nil {
			return err
		}
		switch utils.OutputFormat.Value {
		case utils.OutputPretty:
			return utils.RenderTable(toACLMarkdown(result.AccessControlLists))
		case utils.OutputToml:
			return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
				AccessControlLists []volcengine.AccessControlList `toml:"access_control_lists"`
			}{
				AccessControlLists: result.AccessControlLists,
			})
		case utils.OutputEnv:
			return errors.New(utils.ErrEnvNotSupported)
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	})
}

func Create(ctx context.Context, params Params) error {
	return runConfirmedWorkspaceAction(ctx, params.WorkspaceID, "create network restrictions for", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.CreateAccessControlList(ctx, volcengine.AccessControlListParams{
			WorkspaceID: workspace.WorkspaceID,
			Name:        defaultACLName,
			AclType:     volcengine.ACLTypeAllow,
			IPList:      params.IPList,
		}); err != nil {
			return err
		}
		fmt.Printf("Created network restrictions for project %s.\n", utils.Aqua(workspace.WorkspaceName))
		return nil
	})
}

func Update(ctx context.Context, params Params) error {
	return runConfirmedWorkspaceAction(ctx, params.WorkspaceID, "update network restrictions for", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.ModifyAccessControlList(ctx, volcengine.ModifyAccessControlListParams{
			WorkspaceID: workspace.WorkspaceID,
			Name:        defaultACLName,
			ModifyMode:  params.ModifyMode,
			IPList:      params.IPList,
		}); err != nil {
			return err
		}
		fmt.Printf("Updated network restrictions for project %s.\n", utils.Aqua(workspace.WorkspaceName))
		return nil
	})
}

func Delete(ctx context.Context, workspaceID string) error {
	return runConfirmedWorkspaceAction(ctx, workspaceID, "delete network restrictions for", func(client *volcengine.Client, workspace volcengine.Workspace) error {
		if err := client.DeleteAccessControlList(ctx, workspace.WorkspaceID, defaultACLName); err != nil {
			return err
		}
		fmt.Printf("Deleted network restrictions for project %s.\n", utils.Aqua(workspace.WorkspaceName))
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

func runConfirmedWorkspaceAction(ctx context.Context, workspaceID, action string, run func(*volcengine.Client, volcengine.Workspace) error) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, workspaceID)
	if err != nil {
		return err
	}
	title := fmt.Sprintf("Do you want to %s project %s?", action, utils.Aqua(workspace.WorkspaceID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	return run(volcengine.NewWriteClient(cfg), workspace)
}

func toACLMarkdown(acls []volcengine.AccessControlList) string {
	var table strings.Builder
	table.WriteString(`|NAME|TYPE|IP LIST|
|-|-|-|
`)
	for _, acl := range acls {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|\n",
			acl.Name,
			acl.AclType,
			strings.ReplaceAll(strings.Join(acl.IPList, ", "), "|", "\\|"))
	}
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
