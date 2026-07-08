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
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/list"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/set"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/unset"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	secretsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "secrets",
		Short:   "Manage Supabase secrets",
	}

	secretsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all secrets on Supabase",
		Long:  "List all secrets in the linked project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineSecrets(cmd, "Which project do you want to list secrets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return list.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, secretsListBranchID)
		},
	}

	secretsListBranchID  string
	secretsSetBranchID   string
	secretsUnsetBranchID string

	secretsSetCmd = &cobra.Command{
		Use:   "set <NAME=VALUE> ...",
		Short: "Set a secret(s) on Supabase",
		Long:  "Set a secret(s) to the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineSecrets(cmd, "Which project do you want to set secrets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return set.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, secretsSetBranchID, envFilePath, args, afero.NewOsFs())
		},
	}

	secretsUnsetCmd = &cobra.Command{
		Use:   "unset [NAME] ...",
		Short: "Unset a secret(s) on Supabase",
		Long:  "Unset a secret(s) from the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineSecrets(cmd, "Which project do you want to unset secrets from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return unset.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, secretsUnsetBranchID, args)
		},
	}
)

func preRunVolcengineSecrets(cmd *cobra.Command, prompt string) error {
	linked := cmd.Flags().Changed("linked")
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
}

func init() {
	secretsCmd.PersistentFlags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	secretsListCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	secretsListCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	secretsListCmd.Flags().StringVar(&secretsListBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	secretsSetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	secretsSetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	secretsSetCmd.Flags().StringVar(&secretsSetBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	secretsSetCmd.Flags().StringVar(&envFilePath, "env-file", "", "Read secrets from a .env file.")
	secretsUnsetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	secretsUnsetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	secretsUnsetCmd.Flags().StringVar(&secretsUnsetBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsUnsetCmd)
	rootCmd.AddCommand(secretsCmd)
}
