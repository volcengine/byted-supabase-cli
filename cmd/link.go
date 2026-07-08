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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/link"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

var (
	skipPooler          bool
	linkVolcProjectName string

	linkCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "link",
		Short:   "Link to a Supabase project",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return volcengine.RequireAccessKeysEnv()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
				return err
			}
			if err := parseVolcengineWorkspaceID(ctx); err != nil {
				return err
			}
			fsys := afero.NewOsFs()
			if err := flags.LoadConfig(fsys); err != nil {
				return err
			}
			return link.RunVolcengine(ctx, flags.ProjectRef, fsys)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(os.Stdout, "Finished "+utils.Aqua("byted-supabase-cli link")+".")
		},
	}
)

func parseVolcengineWorkspaceID(ctx context.Context) error {
	if len(flags.ProjectRef) > 0 {
		return nil
	}
	if flags.ProjectRef = viper.GetString("PROJECT_ID"); len(flags.ProjectRef) > 0 {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("missing workspace id. Supply --workspace-id or --project-ref.")
	}
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return errors.Errorf("failed to list volcengine workspaces: %w", err)
	}
	result, err := volcengine.NewClient(cfg).ListWorkspaces(ctx, volcengine.ListWorkspacesParams{
		ProjectName: linkVolcProjectName,
		Limit:       volcenginePromptWorkspaceLimit,
	})
	if err != nil {
		return errors.Errorf("failed to list volcengine workspaces: %w", err)
	}
	if len(result.Workspaces) == 0 {
		return errors.New("no Supabase workspaces found")
	}
	if result.Total > len(result.Workspaces) {
		return errors.Errorf(
			"too many Supabase workspaces found in this region (%d). Supply --workspace-id/--project-ref directly, or narrow the selection with --volc-project-name.",
			result.Total,
		)
	}
	items := make([]utils.PromptItem, len(result.Workspaces))
	for i, workspace := range result.Workspaces {
		items[i] = utils.PromptItem{
			Summary: workspace.WorkspaceID,
			Details: fmt.Sprintf(
				"name: %s, project: %s, region: %s, status: %s",
				workspace.WorkspaceName,
				workspace.ProjectName,
				workspace.RegionID,
				workspace.WorkspaceStatus,
			),
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a workspace:", items)
	if err != nil {
		return err
	}
	flags.ProjectRef = choice.Summary
	fmt.Fprintln(os.Stderr, "Selected workspace:", flags.ProjectRef)
	return nil
}

func init() {
	linkFlags := linkCmd.Flags()
	linkFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Alias of --workspace-id for compatibility with Supabase CLI project ref.")
	markFlagTelemetrySafe(linkFlags.Lookup("project-ref"))
	linkFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	markFlagTelemetrySafe(linkFlags.Lookup("workspace-id"))
	linkFlags.StringVar(&linkVolcProjectName, "volc-project-name", "", "Volcengine ProjectName to filter workspaces during interactive link.")
	// Volcengine link currently does not connect to remote Postgres or sync pooler config.
	// Keep the original Supabase CLI flags here for future DB/pooler adaptation.
	// linkFlags.StringVarP(&dbPassword, "password", "p", "", "Password to your remote Postgres database.")
	// linkFlags.BoolVar(&skipPooler, "skip-pooler", false, "Use direct connection instead of pooler.")
	// For some reason, BindPFlag only works for StringVarP instead of StringP
	rootCmd.AddCommand(linkCmd)
}
