// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package overview

import (
	"context"
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type RunParams struct {
	ProjectName string
}

type volcengineWorkspaceOverview struct {
	Region         string `json:"region" toml:"region" yaml:"region"`
	EngineType     string `json:"engine_type" toml:"engine_type" yaml:"engine_type"`
	WorkspaceTotal int    `json:"workspace_total" toml:"workspace_total" yaml:"workspace_total"`
	RunningTotal   int    `json:"running_total" toml:"running_total" yaml:"running_total"`
	CreatingTotal  int    `json:"creating_total" toml:"creating_total" yaml:"creating_total"`
	UpdatingTotal  int    `json:"updating_total" toml:"updating_total" yaml:"updating_total"`
	StoppedTotal   int    `json:"stopped_total" toml:"stopped_total" yaml:"stopped_total"`
	SuspendedTotal int    `json:"suspended_total" toml:"suspended_total" yaml:"suspended_total"`
	ClosedTotal    int    `json:"closed_total" toml:"closed_total" yaml:"closed_total"`
	ErrorTotal     int    `json:"error_total" toml:"error_total" yaml:"error_total"`
}

func Run(ctx context.Context, params RunParams) error {
	if err := volcengine.RequireAccessKeysEnv(); err != nil {
		return err
	}
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	var overviews []volcengineWorkspaceOverview
	for _, region := range cfgSet.Regions {
		result, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).DescribeSupabaseWorkspaceOverview(ctx, volcengine.WorkspaceOverviewParams{
			ProjectName: params.ProjectName,
		})
		if err != nil {
			return errors.Errorf("failed to describe volcengine workspace overview in region %s: %w", region, err)
		}
		for _, overview := range result.Overviews {
			overviews = append(overviews, mapOverview(region, overview))
		}
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `REGION|ENGINE|WORKSPACES|RUNNING|CREATING|UPDATING|STOPPED|SUSPENDED|CLOSED|ERROR
|-|-|-|-|-|-|-|-|-|-|
`
		for _, overview := range overviews {
			table += fmt.Sprintf(
				"|`%s`|`%s`|`%d`|`%d`|`%d`|`%d`|`%d`|`%d`|`%d`|`%d`|\n",
				overview.Region,
				overview.EngineType,
				overview.WorkspaceTotal,
				overview.RunningTotal,
				overview.CreatingTotal,
				overview.UpdatingTotal,
				overview.StoppedTotal,
				overview.SuspendedTotal,
				overview.ClosedTotal,
				overview.ErrorTotal,
			)
		}
		return utils.RenderTable(table)
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Overviews []volcengineWorkspaceOverview `toml:"overviews"`
		}{
			Overviews: overviews,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, overviews)
}

func mapOverview(region string, overview volcengine.WorkspaceOverview) volcengineWorkspaceOverview {
	return volcengineWorkspaceOverview{
		Region:         region,
		EngineType:     overview.EngineType,
		WorkspaceTotal: overview.WorkspaceTotal,
		RunningTotal:   overview.RunningTotal,
		CreatingTotal:  overview.CreatingTotal,
		UpdatingTotal:  overview.UpdatingTotal,
		StoppedTotal:   overview.StoppedTotal,
		SuspendedTotal: overview.SuspendedTotal,
		ClosedTotal:    overview.ClosedTotal,
		ErrorTotal:     overview.ErrorTotal,
	}
}
