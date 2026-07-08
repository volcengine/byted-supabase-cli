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

package get

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/projects/apiKeys"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
	"github.com/volcengine/byted-supabase-cli/pkg/cast"
)

type RunVolcengineParams struct {
	WorkspaceID string
	BranchID    string
}

func Run(ctx context.Context, branchId string, fsys afero.Fs) error {
	detail, err := getBranchDetail(ctx, branchId)
	if err != nil {
		return err
	}

	if utils.OutputFormat.Value != utils.OutputPretty {
		keys, err := apiKeys.RunGetApiKeys(ctx, detail.Ref)
		if err != nil {
			return err
		}
		pooler, err := utils.GetPoolerConfigPrimary(ctx, detail.Ref)
		if err != nil {
			return err
		}
		envs := toStandardEnvs(detail, pooler, keys)
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, envs)
	}

	table := `|HOST|PORT|USER|PASSWORD|JWT SECRET|POSTGRES VERSION|STATUS|
|-|-|-|-|-|-|-|
` + fmt.Sprintf(
		"|`%s`|`%d`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
		detail.DbHost,
		detail.DbPort,
		*detail.DbUser,
		*detail.DbPass,
		*detail.JwtSecret,
		detail.PostgresVersion,
		detail.Status,
	)

	return utils.RenderTable(table)
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
	result, err := volcengine.NewClient(cfg).DescribeBranchDetail(ctx, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	if result.Branch.BranchID == "" {
		return errors.Errorf("volcengine branch %s not found", params.BranchID)
	}
	if result.WorkspaceName == "" {
		result.WorkspaceName = workspace.WorkspaceName
	}

	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toVolcengineMarkdown(result))
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

func getBranchDetail(ctx context.Context, branchId string) (api.BranchDetailResponse, error) {
	var result api.BranchDetailResponse
	if err := uuid.Validate(branchId); err != nil && !utils.ProjectRefPattern.Match([]byte(branchId)) {
		resp, err := utils.GetSupabase().V1GetABranchWithResponse(ctx, flags.ProjectRef, branchId)
		if err != nil {
			return result, errors.Errorf("failed to find branch: %w", err)
		} else if resp.JSON200 == nil {
			return result, errors.Errorf("unexpected find branch status %d: %s", resp.StatusCode(), string(resp.Body))
		}
		branchId = resp.JSON200.ProjectRef
	}
	resp, err := utils.GetSupabase().V1GetABranchConfigWithResponse(ctx, branchId)
	if err != nil {
		return result, errors.Errorf("failed to get branch: %w", err)
	} else if resp.JSON200 == nil {
		return result, errors.Errorf("unexpected get branch status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	masked := "******"
	if resp.JSON200.DbUser == nil {
		resp.JSON200.DbUser = &masked
	}
	if resp.JSON200.DbPass == nil {
		resp.JSON200.DbPass = &masked
	}
	if resp.JSON200.JwtSecret == nil {
		resp.JSON200.JwtSecret = &masked
	}
	return *resp.JSON200, nil
}

func toStandardEnvs(detail api.BranchDetailResponse, pooler api.SupavisorConfigResponse, keys []api.ApiKeyResponse) map[string]string {
	direct := pgconn.Config{
		Host:     detail.DbHost,
		Port:     cast.UIntToUInt16(cast.IntToUint(detail.DbPort)),
		User:     *detail.DbUser,
		Password: *detail.DbPass,
		Database: "postgres",
	}
	config, err := utils.ParsePoolerURL(pooler.ConnectionString)
	if err != nil {
		fmt.Fprintln(os.Stderr, utils.Yellow("WARNING:"), err)
		config = &direct
	} else {
		config.Password = direct.Password
	}
	envs := apiKeys.ToEnv(keys)
	envs["POSTGRES_URL"] = utils.ToPostgresURL(*config)
	envs["POSTGRES_URL_NON_POOLING"] = utils.ToPostgresURL(direct)
	envs["SUPABASE_URL"] = "https://" + utils.GetSupabaseHost(detail.Ref)
	envs["SUPABASE_JWT_SECRET"] = *detail.JwtSecret
	return envs
}

func toVolcengineMarkdown(result volcengine.DescribeBranchDetailResult) string {
	branch := result.Branch
	var table strings.Builder
	table.WriteString(`|WORKSPACE NAME|WORKSPACE ID|BRANCH ID|NAME|STATUS|DEFAULT|PROTECTED|ARCHIVED|INIT SOURCE|CREATION SOURCE|CREATED AT|UPDATED AT|LAST RESET TIME|
|-|-|-|-|-|-|-|-|-|-|-|-|-|
`)
	fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%t`|`%t`|`%t`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
		strings.ReplaceAll(result.WorkspaceName, "|", "\\|"),
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
		branch.UpdateTime,
		branch.LastResetTime)

	usage := branch.BranchUsage
	table.WriteString(`
|DATA SIZE TOTAL BYTES|DATA SIZE USED BYTES|STORAGE SIZE USED BYTES|FUNCTION CALL NUM|COMPUTE TIME SECONDS|SERVICE TIME SECONDS|LAST RUNNING TIME|STAT TIME|
|-|-|-|-|-|-|-|-|
`)
	fmt.Fprintf(&table, "|`%d`|`%d`|`%d`|`%d`|`%d`|`%d`|`%s`|`%s`|\n",
		usage.DataSizeTotalBytes,
		usage.DataSizeUsedBytes,
		usage.StorageSizeUsedBytes,
		usage.FunctionCallNum,
		usage.ComputeTimeSeconds,
		usage.ServiceTimeSeconds,
		usage.LastRunningTime,
		usage.StatTime)
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
