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

package list

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type linkedProject struct {
	api.V1ProjectWithDatabaseResponse `yaml:",inline"`
	Linked                            bool `json:"linked"`
}

type RunParams struct {
	ProjectRef  string
	Detail      bool
	ProjectName string
	Limit       int
	Offset      int
}

func Run(ctx context.Context, fsys afero.Fs, params ...RunParams) error {
	if err := volcengine.RequireAccessKeysEnv(); err != nil {
		return err
	}
	var runParams RunParams
	if len(params) > 0 {
		runParams = params[0]
	}
	return runVolcengine(ctx, fsys, runParams)
}

func runSupabase(ctx context.Context, fsys afero.Fs) error {
	resp, err := utils.GetSupabase().V1ListAllProjectsWithResponse(ctx)
	if err != nil {
		return errors.Errorf("failed to list projects: %w", err)
	}

	if resp.JSON200 == nil {
		return errors.New("Unexpected error retrieving projects: " + string(resp.Body))
	}

	if err := flags.LoadProjectRef(fsys); err != nil && err != utils.ErrNotLinked {
		fmt.Fprintln(os.Stderr, err)
	}

	var projects []linkedProject
	for _, project := range *resp.JSON200 {
		projects = append(projects, linkedProject{
			V1ProjectWithDatabaseResponse: project,
			Linked:                        project.Id == flags.ProjectRef,
		})
	}

	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `LINKED|ORG ID|REFERENCE ID|NAME|REGION|CREATED AT (UTC)
|-|-|-|-|-|-|
`
		for _, project := range projects {
			table += fmt.Sprintf(
				"|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
				formatBullet(project.Linked),
				project.OrganizationSlug,
				project.Id,
				strings.ReplaceAll(project.Name, "|", "\\|"),
				utils.FormatRegion(project.Region),
				utils.FormatTimestamp(project.CreatedAt),
			)
		}
		return utils.RenderTable(table)
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Projects []linkedProject `toml:"projects"`
		}{
			Projects: projects,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}

	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, projects)
}

type volcengineProject struct {
	Linked              bool   `json:"linked" toml:"linked" yaml:"linked"`
	AccountID           string `json:"account_id" toml:"account_id" yaml:"account_id"`
	ReferenceID         string `json:"reference_id" toml:"reference_id" yaml:"reference_id"`
	Name                string `json:"name" toml:"name" yaml:"name"`
	Region              string `json:"region" toml:"region" yaml:"region"`
	Status              string `json:"status" toml:"status" yaml:"status"`
	EngineType          string `json:"engine_type" toml:"engine_type" yaml:"engine_type"`
	EngineVersion       string `json:"engine_version" toml:"engine_version" yaml:"engine_version"`
	IsAgentPlan         bool   `json:"is_agent_plan" toml:"is_agent_plan" yaml:"is_agent_plan"`
	IsAgentPlanInstance bool   `json:"is_agent_plan_instance" toml:"is_agent_plan_instance" yaml:"is_agent_plan_instance"`
	AgentPlanSeatID     string `json:"agent_plan_seat_id,omitempty" toml:"agent_plan_seat_id,omitempty" yaml:"agent_plan_seat_id,omitempty"`
	CreatedAt           string `json:"created_at" toml:"created_at" yaml:"created_at"`
}

type volcengineProjectDetail struct {
	volcengineProject
	ProjectName              string  `json:"project_name" toml:"project_name" yaml:"project_name"`
	UpdateTime               string  `json:"update_time" toml:"update_time" yaml:"update_time"`
	CreationSource           string  `json:"creation_source" toml:"creation_source" yaml:"creation_source"`
	DeletionProtectionStatus string  `json:"deletion_protection_status" toml:"deletion_protection_status" yaml:"deletion_protection_status"`
	InternetProtocol         string  `json:"internet_protocol" toml:"internet_protocol" yaml:"internet_protocol"`
	DNSVisibility            bool    `json:"dns_visibility" toml:"dns_visibility" yaml:"dns_visibility"`
	SharedPrivateNetwork     bool    `json:"shared_private_network" toml:"shared_private_network" yaml:"shared_private_network"`
	VpcID                    string  `json:"vpc_id" toml:"vpc_id" yaml:"vpc_id"`
	SubnetID                 string  `json:"subnet_id" toml:"subnet_id" yaml:"subnet_id"`
	DeletionProtection       string  `json:"deletion_protection" toml:"deletion_protection" yaml:"deletion_protection"`
	HistoryRetentionHours    int     `json:"history_retention_hours" toml:"history_retention_hours" yaml:"history_retention_hours"`
	PublicConnection         string  `json:"public_connection" toml:"public_connection" yaml:"public_connection"`
	ComputeMinCU             float64 `json:"compute_min_cu" toml:"compute_min_cu" yaml:"compute_min_cu"`
	ComputeMaxCU             float64 `json:"compute_max_cu" toml:"compute_max_cu" yaml:"compute_max_cu"`
	EnableAnalytics          string  `json:"enable_analytics" toml:"enable_analytics" yaml:"enable_analytics"`
	SuspendTimeoutSeconds    int     `json:"suspend_timeout_seconds" toml:"suspend_timeout_seconds" yaml:"suspend_timeout_seconds"`
	DataSizeTotalBytes       int64   `json:"data_size_total_bytes" toml:"data_size_total_bytes" yaml:"data_size_total_bytes"`
	DataSizeUsedBytes        int64   `json:"data_size_used_bytes" toml:"data_size_used_bytes" yaml:"data_size_used_bytes"`
	StorageSizeUsedBytes     int64   `json:"storage_size_used_bytes" toml:"storage_size_used_bytes" yaml:"storage_size_used_bytes"`
	BranchCreatedNum         int64   `json:"branch_created_num" toml:"branch_created_num" yaml:"branch_created_num"`
	FunctionCallNum          int64   `json:"function_call_num" toml:"function_call_num" yaml:"function_call_num"`
	UsageStatTime            string  `json:"usage_stat_time" toml:"usage_stat_time" yaml:"usage_stat_time"`
}

type pageInfo struct {
	region string
	total  int
	count  int
	offset int
	limit  int
}

func runVolcengine(ctx context.Context, fsys afero.Fs, params RunParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	if params.Detail && params.ProjectRef == "" {
		return errors.New("missing workspace id. Supply --workspace-id or --project-ref with --detail.")
	}
	if params.ProjectRef != "" {
		return runVolcengineDetail(ctx, cfgSet, params.ProjectRef)
	}
	linkedWorkspaceID, err := volcengine.LoadLinkedWorkspaceID(fsys)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	var workspaces []volcengine.Workspace
	var pages []pageInfo
	for _, region := range cfgSet.Regions {
		query := volcengine.ListWorkspacesParams{
			ProjectName: params.ProjectName,
			Limit:       params.Limit,
			Offset:      params.Offset,
		}
		client := volcengine.NewClient(cfgSet.ConfigForRegion(region))
		result, err := client.ListWorkspaces(ctx, query)
		if err != nil {
			return errors.Errorf("failed to list volcengine projects in region %s: %w", region, err)
		}
		workspaces = append(workspaces, result.Workspaces...)
		pages = append(pages, pageInfo{
			region: region,
			total:  result.Total,
			count:  len(result.Workspaces),
			offset: params.Offset,
			limit:  params.Limit,
		})
	}
	projects := make([]volcengineProject, 0, len(workspaces))
	for _, workspace := range workspaces {
		project := mapVolcengineProject(workspace)
		project.Linked = workspace.WorkspaceID == linkedWorkspaceID
		projects = append(projects, project)
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `LINKED|ACCOUNT ID|REFERENCE ID|NAME|REGION|STATUS|ENGINE|IS AGENT PLAN|AGENT PLAN SEAT ID|CREATED AT (UTC)
|-|-|-|-|-|-|-|-|-|-|
`
		for _, project := range projects {
			table += fmt.Sprintf(
				"|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%t`|`%s`|`%s`|\n",
				formatBullet(project.Linked),
				project.AccountID,
				project.ReferenceID,
				strings.ReplaceAll(project.Name, "|", "\\|"),
				project.Region,
				project.Status,
				project.EngineType,
				project.IsAgentPlan,
				strings.ReplaceAll(project.AgentPlanSeatID, "|", "\\|"),
				utils.FormatTimestamp(project.CreatedAt),
			)
		}
		if err := utils.RenderTable(table); err != nil {
			return err
		}
		printVolcengineListPaginationHint(pages)
		return nil
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Projects []volcengineProject `toml:"projects"`
		}{
			Projects: projects,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, projects)
}

func printVolcengineListPaginationHint(pages []pageInfo) {
	for _, page := range pages {
		if page.count == 0 || page.total <= page.offset+page.count {
			continue
		}
		nextOffset := page.offset + page.count
		remaining := page.total - nextOffset
		fmt.Fprintf(
			os.Stderr,
			"Showing %d of %d projects in region %s (offset=%d, limit=%d). %d remaining; use --offset %d --limit %d to fetch the next page.\n",
			page.count,
			page.total,
			page.region,
			page.offset,
			page.limit,
			remaining,
			nextOffset,
			page.limit,
		)
	}
}

func runVolcengineDetail(ctx context.Context, cfgSet volcengine.ConfigSet, workspaceID string) error {
	if workspaceID == "" {
		return errors.New("missing workspace id. Supply --workspace-id or --project-ref.")
	}
	var result volcengine.DescribeWorkspaceDetailResult
	var lastErr error
	for _, region := range cfgSet.Regions {
		current, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).DescribeWorkspaceDetail(ctx, workspaceID)
		if err == nil && current.Workspace.WorkspaceID != "" {
			result = current
			break
		}
		if err != nil {
			lastErr = err
		}
	}
	if result.Workspace.WorkspaceID == "" {
		if lastErr != nil {
			return errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
		}
		return errors.Errorf("volcengine project %s not found", workspaceID)
	}
	project := mapVolcengineProjectDetail(result.Workspace)
	if project.EngineType != "" && project.EngineType != volcengine.EngineTypeSupabase {
		return errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, project.EngineType)
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `REFERENCE ID|NAME|REGION|STATUS|ENGINE|PUBLIC CONNECTION|IS AGENT PLAN|AGENT PLAN SEAT ID|CREATED AT (UTC)
|-|-|-|-|-|-|-|-|-|
`
		table += fmt.Sprintf(
			"|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%t`|`%s`|`%s`|\n",
			project.ReferenceID,
			strings.ReplaceAll(project.Name, "|", "\\|"),
			project.Region,
			project.Status,
			project.EngineType,
			project.PublicConnection,
			project.IsAgentPlan,
			strings.ReplaceAll(project.AgentPlanSeatID, "|", "\\|"),
			utils.FormatTimestamp(project.CreatedAt),
		)
		return utils.RenderTable(table)
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Project volcengineProjectDetail `toml:"project"`
		}{
			Project: project,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, project)
}

func mapVolcengineProject(workspace volcengine.Workspace) volcengineProject {
	return volcengineProject{
		AccountID:           workspace.AccountID,
		ReferenceID:         workspace.WorkspaceID,
		Name:                workspace.WorkspaceName,
		Region:              workspace.RegionID,
		Status:              workspace.WorkspaceStatus,
		EngineType:          workspace.EngineType,
		EngineVersion:       workspace.EngineVersion,
		IsAgentPlan:         workspace.IsAgentPlan,
		IsAgentPlanInstance: workspace.IsAgentPlanInstance,
		AgentPlanSeatID:     workspace.AgentPlanSeatID,
		CreatedAt:           workspace.CreateTime,
	}
}

func mapVolcengineProjectDetail(workspace volcengine.Workspace) volcengineProjectDetail {
	return volcengineProjectDetail{
		volcengineProject:        mapVolcengineProject(workspace),
		ProjectName:              workspace.ProjectName,
		UpdateTime:               workspace.UpdateTime,
		CreationSource:           workspace.CreationSource,
		DeletionProtectionStatus: workspace.DeletionProtectionStatus,
		InternetProtocol:         workspace.InternetProtocol,
		DNSVisibility:            workspace.DNSVisibility,
		SharedPrivateNetwork:     workspace.SharedPrivateNetwork,
		VpcID:                    workspace.VpcID,
		SubnetID:                 workspace.SubnetID,
		DeletionProtection:       workspace.WorkspaceSetting.DeletionProtection,
		HistoryRetentionHours:    workspace.WorkspaceSetting.HistoryRetentionHours,
		PublicConnection:         workspace.WorkspaceSetting.PublicConnection,
		ComputeMinCU:             workspace.ComputeSettings.AutoScalingLimitMinCU,
		ComputeMaxCU:             workspace.ComputeSettings.AutoScalingLimitMaxCU,
		EnableAnalytics:          workspace.ComputeSettings.EnableAnalytics,
		SuspendTimeoutSeconds:    workspace.ComputeSettings.SuspendTimeoutSeconds,
		DataSizeTotalBytes:       workspace.WorkspaceUsage.DataSizeTotalBytes,
		DataSizeUsedBytes:        workspace.WorkspaceUsage.DataSizeUsedBytes,
		StorageSizeUsedBytes:     workspace.WorkspaceUsage.StorageSizeUsedBytes,
		BranchCreatedNum:         workspace.WorkspaceUsage.BranchCreatedNum,
		FunctionCallNum:          workspace.WorkspaceUsage.FunctionCallNum,
		UsageStatTime:            workspace.WorkspaceUsage.StatTime,
	}
}

func formatBullet(value bool) string {
	if value {
		return "  ●"
	}
	return " "
}
