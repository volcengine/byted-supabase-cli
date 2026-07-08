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
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/orgs/create"
	"github.com/volcengine/byted-supabase-cli/internal/orgs/list"
)

var (
	orgsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "orgs",
		Short:   "Manage Supabase organizations",
	}

	orgsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all organizations",
		Long:  "List all organizations the logged-in user belongs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return list.Run(cmd.Context())
		},
	}

	orgsCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create an organization",
		Long:  "Create an organization for the logged-in user.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return create.Run(cmd.Context(), args[0])
		},
	}
)

func init() {
	orgsCmd.AddCommand(orgsListCmd)
	orgsCmd.AddCommand(orgsCreateCmd)
	// rootCmd.AddCommand(orgsCmd) // Volcengine has no equivalent organization resource; code retained but not registered
}
