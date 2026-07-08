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
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/postgresConfig/delete"
	"github.com/volcengine/byted-supabase-cli/internal/postgresConfig/get"
	"github.com/volcengine/byted-supabase-cli/internal/postgresConfig/update"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	postgresCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "postgres-config",
		Short:   "Manage Postgres database config",
	}

	postgresConfigGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get the current Postgres database config overrides",
		RunE: func(cmd *cobra.Command, args []string) error {
			return get.Run(cmd.Context(), flags.ProjectRef, afero.NewOsFs())
		},
	}

	postgresConfigUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update Postgres database config",
		Long: `Overriding the default Postgres config could result in unstable database behavior.
Custom configuration also overrides the optimizations generated based on the compute add-ons in use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return update.Run(cmd.Context(), flags.ProjectRef, postgresConfigValues, postgresConfigUpdateReplaceMode, noRestart, afero.NewOsFs())
		},
	}

	postgresConfigDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete specific Postgres database config overrides",
		Long:  "Delete specific config overrides, reverting them to their default values.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return delete.Run(cmd.Context(), flags.ProjectRef, postgresConfigKeysToDelete, noRestart, afero.NewOsFs())
		},
	}

	postgresConfigValues            []string
	postgresConfigUpdateReplaceMode bool
	postgresConfigKeysToDelete      []string
	noRestart                       bool
)

func init() {
	postgresCmd.PersistentFlags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project.")
	postgresCmd.AddCommand(postgresConfigGetCmd)
	postgresCmd.AddCommand(postgresConfigUpdateCmd)
	postgresCmd.AddCommand(postgresConfigDeleteCmd)

	updateFlags := postgresConfigUpdateCmd.Flags()
	updateFlags.StringSliceVar(&postgresConfigValues, "config", []string{}, "Config overrides specified as a 'key=value' pair")
	updateFlags.BoolVar(&postgresConfigUpdateReplaceMode, "replace-existing-overrides", false, "If true, replaces all existing overrides with the ones provided. If false (default), merges existing overrides with the ones provided.")
	updateFlags.BoolVar(&noRestart, "no-restart", false, "Do not restart the database after updating config.")

	deleteFlags := postgresConfigDeleteCmd.Flags()
	deleteFlags.StringSliceVar(&postgresConfigKeysToDelete, "config", []string{}, "Config keys to delete (comma-separated)")
	deleteFlags.BoolVar(&noRestart, "no-restart", false, "Do not restart the database after deleting config.")

	// rootCmd.AddCommand(postgresCmd) // Volcengine has no equivalent Postgres config override API; code retained but not registered
}
