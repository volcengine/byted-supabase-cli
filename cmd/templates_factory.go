// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func newTemplatesCommand(ctx RuntimeContext) *cobra.Command {
	var options sandboxWorkspaceOptions
	cmd := &cobra.Command{GroupID: groupManagementAPI, Use: "templates", Short: "Manage Sandbox templates"}
	list := &cobra.Command{
		Use: "list", Aliases: []string{"ls"}, Short: "List Sandbox templates", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := sandboxClient(cmd)
			if err != nil {
				return err
			}
			result, err := client.DescribeBaasSandboxTemplates(cmd.Context(), strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.BranchID))
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSandboxWorkspaceFlags(list, &options)
	cmd.AddCommand(list)
	return cmd
}
