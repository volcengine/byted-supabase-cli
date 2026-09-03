// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/update"
)

type updateOptions struct {
	CheckOnly bool
	Force     bool
	JSON      bool
}

func updateCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newUpdateCommand}}
}

func newUpdateCommand(ctx RuntimeContext) *cobra.Command {
	var options updateOptions
	cmd := &cobra.Command{
		Use: "update", Short: "Update byted-supabase-cli to the latest version",
		Long: `Update byted-supabase-cli to the latest version published on npm.

Detects the installation method automatically:
  - npm install: runs npm install -g @byted-supabase/cli@<version>
  - manual/other: prints the command to run manually

Use --check to only check for updates without installing.
Use --json for structured output (for scripts and AI agents).`,
		Example: "  byted-supabase-cli update\n  byted-supabase-cli update --check\n  byted-supabase-cli update --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return update.Run(cmd.Context(), update.Params{CheckOnly: options.CheckOnly, Force: options.Force, JSON: options.JSON}, ctx.Output, ctx.Stderr)
		},
	}
	cmd.Flags().BoolVar(&options.CheckOnly, "check", false, "Only check for updates, do not install.")
	cmd.Flags().BoolVar(&options.Force, "force", false, "Reinstall even if already up to date.")
	cmd.Flags().BoolVar(&options.JSON, "json", false, "Output structured JSON (for scripts and AI agents).")
	return cmd
}
