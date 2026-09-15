// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/list"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/set"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/unset"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

type secretsOptions struct {
	WorkspaceID string
	Linked      bool
	BranchID    string
}

func secretsCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newSecretsCommand}}
}

func newSecretsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "secrets",
		Short:   "Manage Supabase Secrets",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newSecretsListCommand(ctx),
		newSecretsSetCommand(ctx),
		newSecretsUnsetCommand(ctx),
	)
	return cmd
}

func addSecretsWorkspaceFlags(cmd *cobra.Command, options *secretsOptions) {
	cmd.Flags().BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("project-ref"))
	markFlagTelemetrySafe(cmd.Flags().Lookup("workspace-id"))
}

func prepareSecretsWorkspace(cmd *cobra.Command, options *secretsOptions, prompt string) error {
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if cmd.Flags().Changed("linked") && options.Linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	previous := flags.ProjectRef
	if options.WorkspaceID != "" {
		flags.ProjectRef = options.WorkspaceID
	}
	err := loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
	if err == nil {
		options.WorkspaceID = flags.ProjectRef
	}
	flags.ProjectRef = previous
	return err
}

func newSecretsListCommand(ctx RuntimeContext) *cobra.Command {
	var options secretsOptions
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all secrets on Supabase",
		Long:  "List all secrets in the linked project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareSecretsWorkspace(cmd, &options, "Which project do you want to list secrets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return list.RunVolcengine(cmd.Context(), client, options.WorkspaceID, options.BranchID)
		},
	}
	addSecretsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	_ = ctx
	return cmd
}

func newSecretsSetCommand(ctx RuntimeContext) *cobra.Command {
	var options secretsOptions
	var envFile string
	cmd := &cobra.Command{
		Use:   "set <NAME=VALUE> ...",
		Short: "Set a secret(s) on Supabase",
		Long:  "Set a secret(s) to the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareSecretsWorkspace(cmd, &options, "Which project do you want to set secrets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return set.RunVolcengine(cmd.Context(), client, options.WorkspaceID, options.BranchID, envFile, args, afero.NewOsFs())
		},
	}
	addSecretsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&envFile, "env-file", "", "Read secrets from a .env file.")
	_ = ctx
	return cmd
}

func newSecretsUnsetCommand(ctx RuntimeContext) *cobra.Command {
	var options secretsOptions
	cmd := &cobra.Command{
		Use:   "unset [NAME] ...",
		Short: "Unset a secret(s) on Supabase",
		Long:  "Unset a secret(s) from the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareSecretsWorkspace(cmd, &options, "Which project do you want to unset secrets from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			return unset.RunVolcengine(cmd.Context(), client, options.WorkspaceID, options.BranchID, args)
		},
	}
	addSecretsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	_ = ctx
	return cmd
}
