// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

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

type linkOptions struct {
	WorkspaceID string
	ProjectName string
}

func newLinkCommand(ctx RuntimeContext) *cobra.Command {
	var options linkOptions
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "link",
		Short:   "Link to a Supabase Project",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return volcengine.RequireAccessKeysEnv()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), ctx.FS, false); err != nil {
				return err
			}
			if err := parseVolcengineWorkspaceIDFactory(cmd.Context(), ctx.FS, &options); err != nil {
				return err
			}
			if err := flags.LoadConfig(ctx.FS); err != nil {
				return err
			}
			return link.RunVolcengine(cmd.Context(), options.WorkspaceID, ctx.FS)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(ctx.Output, "Finished "+utils.Aqua("byted-supabase-cli link")+".")
		},
	}
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Alias of --workspace-id for compatibility with Supabase CLI project ref.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("project-ref"))
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("workspace-id"))
	cmd.Flags().StringVar(&options.ProjectName, "volc-project-name", "", "Volcengine ProjectName to filter workspaces during interactive link.")
	return cmd
}

func parseVolcengineWorkspaceIDFactory(ctx context.Context, fsys afero.Fs, options *linkOptions) error {
	if options.WorkspaceID != "" {
		return nil
	}
	if options.WorkspaceID = viper.GetString("PROJECT_ID"); options.WorkspaceID != "" {
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
		ProjectName: options.ProjectName,
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
	options.WorkspaceID = choice.Summary
	fmt.Fprintln(os.Stderr, "Selected workspace:", options.WorkspaceID)
	return nil
}
