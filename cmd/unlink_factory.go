// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/unlink"
)

func newUnlinkCommand(ctx RuntimeContext) *cobra.Command {
	return &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "unlink",
		Short:   "Unlink a Supabase Project or Volcengine Workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			return unlink.Run(cmd.Context(), ctx.FS)
		},
	}
}
