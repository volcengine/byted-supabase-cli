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

package apiKeys

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/oapi-codegen/nullable"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type RunVolcengineParams struct {
	WorkspaceID string
	BranchID    string
	Limit       int
	Offset      int
	Fsys        afero.Fs
}

func Run(ctx context.Context, projectRef string, fsys afero.Fs) error {
	keys, err := RunGetApiKeys(ctx, projectRef)
	if err != nil {
		return err
	}

	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `|NAME|KEY VALUE|
|-|-|
`
		for _, entry := range keys {
			k := strings.ReplaceAll(entry.Name, "|", "\\|")
			v := toValue(entry.ApiKey)
			table += fmt.Sprintf("|`%s`|`%s`|\n", k, v)
		}

		return utils.RenderTable(table)
	case utils.OutputToml, utils.OutputEnv:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, ToEnv(keys))
	}

	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, keys)
}

func RunVolcengine(ctx context.Context, params RunVolcengineParams) error {
	fsys := params.Fsys
	if fsys == nil {
		fsys = afero.NewOsFs()
	}
	cfgSet, err := volcengine.LoadConfigSetFromEnvWithLinkedRegion(fsys)
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, params.WorkspaceID)
	if err != nil {
		return err
	}
	client := volcengine.NewClient(cfg)
	branchID, err := client.ResolveDefaultBranchID(ctx, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	result, err := client.DescribeAPIKeys(ctx, volcengine.DescribeAPIKeysParams{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
		Limit:       params.Limit,
		Offset:      params.Offset,
	})
	if err != nil {
		return err
	}
	keys := mapVolcengineAPIKeys(result.APIKeys)
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `|NAME|KEY VALUE|
|-|-|
`
		for _, entry := range keys {
			table += fmt.Sprintf("|`%s`|`%s`|\n", strings.ReplaceAll(entry.Name, "|", "\\|"), entry.Key)
		}
		return utils.RenderTable(table)
	case utils.OutputToml, utils.OutputEnv:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, toVolcengineEnv(keys))
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, keys)
}

func RunGetApiKeys(ctx context.Context, projectRef string) ([]api.ApiKeyResponse, error) {
	resp, err := utils.GetSupabase().V1GetProjectApiKeysWithResponse(ctx, projectRef, &api.V1GetProjectApiKeysParams{})
	if err != nil {
		return nil, errors.Errorf("failed to get api keys: %w", err)
	} else if resp.JSON200 == nil {
		return nil, errors.Errorf("unexpected get api keys status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return *resp.JSON200, nil
}

func ToEnv(keys []api.ApiKeyResponse) map[string]string {
	envs := make(map[string]string, len(keys))
	for _, entry := range keys {
		name := strings.ToUpper(entry.Name)
		key := fmt.Sprintf("SUPABASE_%s_KEY", name)
		envs[key] = toValue(entry.ApiKey)
	}
	return envs
}

func toValue(v nullable.Nullable[string]) string {
	if value, err := v.Get(); err == nil {
		return value
	}
	return "******"
}

type volcengineAPIKey struct {
	Name       string `json:"name" toml:"name" yaml:"name"`
	Key        string `json:"api_key" toml:"api_key" yaml:"api_key"`
	Type       string `json:"type" toml:"type" yaml:"type"`
	CreateTime string `json:"created_at" toml:"created_at" yaml:"created_at"`
}

func mapVolcengineAPIKeys(keys []volcengine.APIKey) []volcengineAPIKey {
	result := make([]volcengineAPIKey, 0, len(keys))
	for _, key := range keys {
		name := key.Name
		if name == "" {
			name = volcengineAPIKeyName(key.Type)
		}
		result = append(result, volcengineAPIKey{
			Name:       name,
			Key:        key.Key,
			Type:       key.Type,
			CreateTime: key.CreateTime,
		})
	}
	return result
}

func volcengineAPIKeyName(keyType string) string {
	switch keyType {
	case "Public":
		return "anon"
	case "Service":
		return "service_role"
	default:
		return strings.ToLower(keyType)
	}
}

func toVolcengineEnv(keys []volcengineAPIKey) map[string]string {
	envs := make(map[string]string, len(keys))
	for _, entry := range keys {
		name := strings.ToUpper(entry.Name)
		name = strings.ReplaceAll(name, "-", "_")
		name = strings.ReplaceAll(name, " ", "_")
		envs[fmt.Sprintf("SUPABASE_%s_KEY", name)] = entry.Key
	}
	return envs
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
		return workspace, cfg, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("volcengine project %s not found", workspaceID)
}
