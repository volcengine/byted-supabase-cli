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
	"os"

	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	logoutProfile string
	logoutAll     bool

	logoutCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "logout",
		Short:   "Log out from Volcengine",
		Long: `Remove locally cached Console Login credentials and delete console-login profiles.

This is a purely local operation. It deletes cached STS token files from disk
and deletes console-login profiles from the CLI configuration. It does not delete
AK/SK profiles.`,
		Example: `byted-supabase-cli logout
byted-supabase-cli logout --profile dev
byted-supabase-cli logout --all`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return volcengine.RunConsoleLogout(volcengine.ConsoleLogoutParams{
				Profile: logoutProfile,
				All:     logoutAll,
			}, os.Stdout)
		},
	}
)

func init() {
	logoutFlags := logoutCmd.Flags()
	logoutFlags.StringVarP(&logoutProfile, "profile", "p", "default", "Volcengine profile name.")
	logoutFlags.BoolVar(&logoutAll, "all", false, "Log out all console-login profiles and remove all cached login credentials.")
	rootCmd.AddCommand(logoutCmd)
}
