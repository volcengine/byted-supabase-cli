// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	dbconnection "github.com/volcengine/byted-supabase-cli/internal/db/connection"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

type inspectOptions struct {
	WorkspaceID string
	BranchID    string
	Linked      bool
	DBURL       string
	OutputDir   string
}

type inspectRunnerFactory func(context.Context, pgconn.Config, afero.Fs, ...func(*pgx.ConnConfig)) error

func newInspectCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "inspect",
		Short:   "Tools to Inspect Your Supabase Project",
	}
	SetAuthPolicy(cmd, AuthPolicyNone)
	cmd.AddCommand(
		newInspectDBCommand(ctx),
		newInspectReportCommand(ctx),
	)
	return cmd
}

func addInspectTargetFlags(cmd *cobra.Command, options *inspectOptions) {
	cmd.Flags().StringVar(&options.DBURL, "db-url", "", "Inspect the database specified by the connection string (must be percent-encoded).")
	cmd.Flags().BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --db-url is omitted.")
	cmd.MarkFlagsMutuallyExclusive("db-url", "linked")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the default branch.")
}

func preRunInspectFactory(cmd *cobra.Command, fsys afero.Fs, options *inspectOptions) error {
	direct := cmd.Flags().Changed("db-url")
	linked := cmd.Flags().Changed("linked")
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if direct && (linked || workspace || cmd.Flags().Changed("branch-id")) {
		return errors.New("--db-url cannot be combined with --linked, --workspace-id, --project-ref, or --branch-id")
	}
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	if direct {
		return nil
	}
	previous := flags.ProjectRef
	if options.WorkspaceID != "" {
		flags.ProjectRef = options.WorkspaceID
	}
	err := loadOrPromptVolcengineProjectRef(cmd.Context(), fsys, "Which project do you want to inspect?")
	if options.WorkspaceID == "" {
		options.WorkspaceID = flags.ProjectRef
	}
	flags.ProjectRef = previous
	return err
}

func runInspectFactory(cmd *cobra.Command, fsys afero.Fs, options *inspectOptions, runner inspectRunnerFactory) error {
	config := flags.DbConfig
	if !cmd.Flags().Changed("db-url") {
		client, err := volcengineClientFromAuth(cmd)
		if err != nil {
			return err
		}
		config, err = dbconnection.GetVolcengineConfig(cmd.Context(), client, dbconnection.VolcengineParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID})
		if err != nil {
			return err
		}
	}
	return runner(cmd.Context(), config, fsys)
}

func isVolcengineInspectCommand(cmd *cobra.Command) bool {
	if !hasCommandAncestor(cmd, "inspect") {
		return false
	}
	switch cmd.Name() {
	case "db-stats", "replication-slots", "locks", "blocking", "outliers",
		"calls", "index-stats", "long-running-queries", "bloat",
		"role-stats", "vacuum-stats", "table-stats", "traffic-profile", "report":
		return true
	default:
		return false
	}
}
