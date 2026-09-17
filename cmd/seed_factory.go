// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/seed/buckets"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

type seedOptions struct {
	WorkspaceID string
	Linked      bool
}

func seedCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newSeedCommand}}
}

func newSeedCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "seed",
		Short:   "Manage Supabase Seed Data from " + utils.ConfigPath,
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(newSeedBucketsCommand(ctx))
	return cmd
}

func addSeedWorkspaceFlags(cmd *cobra.Command, options *seedOptions) {
	flags := cmd.Flags()
	flags.BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to the linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func prepareSeedWorkspace(cmd *cobra.Command, fsys afero.Fs, options *seedOptions) error {
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if cmd.Flags().Changed("linked") && options.Linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	workspaceID, err := (WorkspaceResolver{FS: fsys}).Resolve(cmd.Context(), options.WorkspaceID, options.Linked)
	if err == nil {
		options.WorkspaceID = workspaceID
	}
	return err
}

func newSeedBucketsCommand(ctx RuntimeContext) *cobra.Command {
	options := &seedOptions{}
	var branchID string
	cmd := &cobra.Command{
		Use:   "buckets",
		Short: "Seed buckets declared in [storage.buckets]",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareSeedWorkspace(cmd, ctx.FS, options)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fsys := afero.NewOsFs()
			if err := flags.LoadConfig(fsys); err != nil {
				return err
			}
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return buckets.RunVolcengine(cmd.Context(), client, options.WorkspaceID, branchID, true, fsys)
		},
	}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	addSeedWorkspaceFlags(cmd, options)
	_ = ctx
	return cmd
}
