// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/unlink"
)

func newUnlinkCommand(ctx RuntimeContext) *cobra.Command {
	return &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "unlink",
		Short:   "Remove the current link to a Supabase branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, err := cmd.Flags().GetBool("yes")
			if err != nil {
				yes = viper.GetBool("YES")
			}
			return unlink.Run(cmd.Context(), ctx.FS, yes)
		},
	}
}
