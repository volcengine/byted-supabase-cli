// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/volcengine/byted-supabase-cli/internal/update"
)

var (
	updateCheckOnly bool
	updateForce     bool
	updateJSON      bool

	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update byted-supabase-cli to the latest version",
		Long: `Update byted-supabase-cli to the latest version published on npm.

Detects the installation method automatically:
  - npm install: runs npm install -g @byted-supabase/cli@<version>
  - manual/other: prints the command to run manually

Use --check to only check for updates without installing.
Use --json for structured output (for scripts and AI agents).`,
		Example: `  byted-supabase-cli update
  byted-supabase-cli update --check
  byted-supabase-cli update --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return update.Run(cmd.Context(), update.Params{
				CheckOnly: updateCheckOnly,
				Force:     updateForce,
				JSON:      updateJSON,
			}, os.Stdout, os.Stderr)
		},
	}
)

func init() {
	flags := updateCmd.Flags()
	flags.BoolVar(&updateCheckOnly, "check", false, "Only check for updates, do not install.")
	flags.BoolVar(&updateForce, "force", false, "Reinstall even if already up to date.")
	flags.BoolVar(&updateJSON, "json", false, "Output structured JSON (for scripts and AI agents).")
	rootCmd.AddCommand(updateCmd)
}

// updateCommandMounted reports whether the update command is still mounted on
// the root after distribution assembly. Distributions whose release channel is
// not the Volcengine npm registry prune update via the distribution seam; when
// they do, the upgrade check and its nag are pointless (they would suggest a
// command that no longer exists), so the command tree is the single source of
// truth for whether to run them — no separate toggle to keep in sync.
//
// Matched by identity, not name: a distribution may mount its own command also
// named "update" against a different release channel, and that must not
// re-enable the Volcengine npm check.
func updateCommandMounted() bool {
	for _, c := range rootCmd.Commands() {
		if c == updateCmd {
			return true
		}
	}
	return false
}
