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
	"github.com/volcengine/byted-supabase-cli/internal/ssl_enforcement/get"
	"github.com/volcengine/byted-supabase-cli/internal/ssl_enforcement/update"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	sslEnforcementCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "ssl-enforcement",
		Short:   "Manage SSL enforcement configuration",
	}

	dbEnforceSsl bool
	dbDisableSsl bool

	sslEnforcementUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update SSL enforcement configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dbEnforceSsl && !dbDisableSsl {
				return errors.New("enable/disable not specified")
			}
			return update.Run(cmd.Context(), flags.ProjectRef, dbEnforceSsl, afero.NewOsFs())
		},
	}

	sslEnforcementGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get the current SSL enforcement configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return get.Run(cmd.Context(), flags.ProjectRef, afero.NewOsFs())
		},
	}
)

func init() {
	sslEnforcementCmd.PersistentFlags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project.")
	sslEnforcementUpdateCmd.Flags().BoolVar(&dbEnforceSsl, "enable-db-ssl-enforcement", false, "Whether the DB should enable SSL enforcement for all external connections.")
	sslEnforcementUpdateCmd.Flags().BoolVar(&dbDisableSsl, "disable-db-ssl-enforcement", false, "Whether the DB should disable SSL enforcement for all external connections.")
	sslEnforcementUpdateCmd.MarkFlagsMutuallyExclusive("enable-db-ssl-enforcement", "disable-db-ssl-enforcement")
	sslEnforcementCmd.AddCommand(sslEnforcementUpdateCmd)
	sslEnforcementCmd.AddCommand(sslEnforcementGetCmd)

	// rootCmd.AddCommand(sslEnforcementCmd) // Volcengine has no equivalent SSL enforcement policy API; code retained but not registered
}
