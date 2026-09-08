// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type logoutOptions struct {
	Profile string
	All     bool
}

func newLogoutCommand(ctx RuntimeContext) *cobra.Command {
	var options logoutOptions
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "logout",
		Short:   "Log Out from Volcengine",
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
				Profile: options.Profile,
				All:     options.All,
			}, ctx.Output)
		},
	}
	SetAuthPolicy(cmd, AuthPolicyNone)
	cmd.Flags().StringVarP(&options.Profile, "profile", "p", "default", "Volcengine profile name.")
	cmd.Flags().BoolVar(&options.All, "all", false, "Log out all console-login profiles and remove all cached login credentials.")
	return cmd
}
