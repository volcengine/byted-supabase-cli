// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package list

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type Params struct {
	WorkspaceID     string
	BranchID        string
	ComputeID       string
	ActionName      string
	Status          string
	CreateTimeStart string
	CreateTimeEnd   string
	Limit           int
	Offset          int
}

type Result struct {
	Total      int                    `json:"total" toml:"total" yaml:"total"`
	Limit      int                    `json:"limit" toml:"limit" yaml:"limit"`
	Offset     int                    `json:"offset" toml:"offset" yaml:"offset"`
	Operations []volcengine.Operation `json:"operations" toml:"operations" yaml:"operations"`
}

func Run(ctx context.Context, params Params) error {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	client := volcengine.NewClient(cfg)
	if params.WorkspaceID != "" {
		workspace, err := client.DescribeWorkspaceDetail(ctx, params.WorkspaceID)
		if err != nil {
			return errors.Errorf("failed to describe volcengine project %s: %w", params.WorkspaceID, err)
		}
		if workspace.Workspace.WorkspaceID == "" {
			return errors.Errorf("volcengine project %s not found", params.WorkspaceID)
		}
		if workspace.Workspace.EngineType != "" && workspace.Workspace.EngineType != volcengine.EngineTypeSupabase {
			return errors.Errorf("workspace %s is %s, expected Supabase", params.WorkspaceID, workspace.Workspace.EngineType)
		}
	}
	response, err := client.DescribeOperations(ctx, volcengine.DescribeOperationsParams{
		WorkspaceID:     params.WorkspaceID,
		BranchID:        params.BranchID,
		ComputeID:       params.ComputeID,
		ActionName:      params.ActionName,
		Status:          params.Status,
		CreateTimeStart: params.CreateTimeStart,
		CreateTimeEnd:   params.CreateTimeEnd,
		Limit:           params.Limit,
		Offset:          params.Offset,
	})
	if err != nil {
		return err
	}
	result := Result{
		Total:      response.Total,
		Limit:      params.Limit,
		Offset:     params.Offset,
		Operations: response.Operations,
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		if params.Offset+len(result.Operations) < result.Total {
			fmt.Fprintf(os.Stderr, "Showing %d-%d of %d operations. Use --offset %d to view the next page.\n",
				params.Offset+1, params.Offset+len(result.Operations), result.Total, params.Offset+params.Limit)
		}
		return utils.RenderTable(toMarkdown(result.Operations))
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func toMarkdown(operations []volcengine.Operation) string {
	var table strings.Builder
	table.WriteString(`|OPERATION ID|ACTION|STATUS|WORKSPACE ID|BRANCH ID|COMPUTE ID|CREATED AT|FINISHED AT|DURATION|
|-|-|-|-|-|-|-|-|-|
`)
	for _, operation := range operations {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|\n",
			operation.OperationID,
			strings.ReplaceAll(operation.ActionName, "|", "\\|"),
			operation.ActionStatus,
			operation.WorkspaceID,
			operation.BranchID,
			operation.ComputeID,
			operation.CreateTime,
			operation.FinishTime,
			operation.DurationTime,
		)
	}
	return table.String()
}
