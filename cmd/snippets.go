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
	"github.com/volcengine/byted-supabase-cli/internal/snippets/download"
	"github.com/volcengine/byted-supabase-cli/internal/snippets/list"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	snippetsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "snippets",
		Short:   "Manage Supabase SQL snippets",
	}

	snippetsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all SQL snippets",
		Long:  "List all SQL snippets of the linked project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return list.Run(cmd.Context(), afero.NewOsFs())
		},
	}

	snippetsDownloadCmd = &cobra.Command{
		Use:   "download <snippet-id>",
		Short: "Download contents of a SQL snippet",
		Long:  "Download contents of the specified SQL snippet.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return download.Run(cmd.Context(), args[0], afero.NewOsFs())
		},
	}
)

func init() {
	snippetsCmd.PersistentFlags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project.")
	snippetsCmd.AddCommand(snippetsListCmd)
	snippetsCmd.AddCommand(snippetsDownloadCmd)
	// Ve Studio currently does not provide the SQL snippets capability exposed
	// by Supabase Platform, so keep the implementation but do not expose it.
	// rootCmd.AddCommand(snippetsCmd)
}
