// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"

	"github.com/volcengine/byted-supabase-cli/internal/skills"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

var (
	skillsForce bool
	skillsJSON  bool

	skillsCmd = &cobra.Command{
		Use:   "skills",
		Short: "Manage the byted-supabase agent skill",
		Long: `Install and inspect the byted-supabase skill (for Claude Code and other agents).

The skill is published to skills.volces.com and installed globally via the
external ` + "`skills`" + ` tool (requires Node.js / npx). It is also kept in sync
automatically by ` + "`byted-supabase-cli update`" + `.`,
	}

	skillsInstallCmd = &cobra.Command{
		Use:   "install",
		Short: "Install or update the byted-supabase skill",
		Long: `Install or update the byted-supabase skill, pinning it to this CLI version.

Runs: npx -y skills add <source> -s ` + skills.Name() + ` -g -y

Use --force to reinstall even if it is already in sync.
Use --json for structured output (for scripts and AI agents).`,
		Example: `  byted-supabase-cli skills install
  byted-supabase-cli skills install --force
  byted-supabase-cli skills install --json`,
		Aliases: []string{"update", "sync"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillsInstall(cmd.Context(), skillsForce, skillsJSON, os.Stdout, os.Stderr)
		},
	}

	skillsStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the installed skill version and sync state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillsStatus(skillsJSON, os.Stdout, os.Stderr)
		},
	}
)

func init() {
	skillsInstallCmd.Flags().BoolVar(&skillsForce, "force", false, "Reinstall even if already in sync.")
	skillsInstallCmd.Flags().BoolVar(&skillsJSON, "json", false, "Output structured JSON.")
	skillsStatusCmd.Flags().BoolVar(&skillsJSON, "json", false, "Output structured JSON.")

	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsStatusCmd)
	rootCmd.AddCommand(skillsCmd)
}

func runSkillsInstall(ctx context.Context, force, asJSON bool, stdout, stderr io.Writer) error {
	version := strings.TrimSpace(utils.Version)

	if !force && skills.IsSynced(version) {
		if asJSON {
			return writeJSON(stdout, map[string]any{
				"ok": true, "action": "in_sync", "skill": skills.Name(), "version": version,
				"message": fmt.Sprintf("%s skill is already in sync", skills.Name()),
			})
		}
		fmt.Fprintf(stderr, "✓ %s skill is already in sync (use --force to reinstall)\n", skills.Name())
		return nil
	}

	if !asJSON {
		fmt.Fprintf(stderr, "Installing %s skill from %s ...\n", skills.Name(), skills.Source())
	}
	res := skills.Sync(ctx, skills.SyncOptions{Version: version, Force: force})

	if res.Action == "failed" {
		msg := fmt.Sprintf("skill install failed: %s", res.Err)
		if asJSON {
			_ = writeJSON(stdout, map[string]any{
				"ok": false, "skill": skills.Name(),
				"error": map[string]any{"message": msg, "detail": res.Detail},
			})
		} else if res.Detail != "" {
			fmt.Fprintln(stderr, res.Detail)
		}
		return errors.New(msg)
	}

	if asJSON {
		env := map[string]any{
			"ok": true, "action": "synced", "skill": res.Skill, "version": res.Version,
			"message": fmt.Sprintf("%s skill installed", res.Skill),
		}
		if res.Err != nil {
			env["warning"] = res.Err.Error()
		}
		return writeJSON(stdout, env)
	}
	if res.Err != nil {
		fmt.Fprintf(stderr, "⚠ %s skill installed (state not saved: %v)\n", res.Skill, res.Err)
		return nil
	}
	fmt.Fprintf(stderr, "✓ %s skill installed\n", res.Skill)
	return nil
}

func runSkillsStatus(asJSON bool, stdout, stderr io.Writer) error {
	version := strings.TrimSpace(utils.Version)
	state, ok, err := skills.ReadState()
	if err != nil {
		return errors.Errorf("failed to read skills state: %w", err)
	}

	// "installed" is keyed off the presence of a state record (written only on a
	// successful install), NOT off a non-empty version: a dev build stamps an
	// empty version but the skill is still installed. The version only drives
	// drift detection, which is disabled when either side is unversioned.
	installed := ok && state.Skill != ""
	versionTracked := installed && state.Version != "" && version != ""
	inSync := versionTracked && skills.IsSynced(version)

	if asJSON {
		env := map[string]any{
			"ok": true, "skill": skills.Name(), "cli_version": version,
			"installed": installed, "in_sync": inSync, "version_tracked": versionTracked,
		}
		if installed {
			env["skill_version"] = state.Version
			env["source"] = state.Source
			env["updated_at"] = state.UpdatedAt
		}
		return writeJSON(stdout, env)
	}

	if !installed {
		fmt.Fprintf(stderr, "%s skill: not installed\n", skills.Name())
		fmt.Fprintln(stderr, "Install it with: byted-supabase-cli skills install")
		return nil
	}
	if !versionTracked {
		// Dev build (CLI or skill version unknown): installed, but we can't
		// compare versions, so don't claim in-sync / out-of-date.
		fmt.Fprintf(stderr, "%s skill: installed (version not tracked on this build)\n", skills.Name())
		return nil
	}
	status := "in sync"
	if !inSync {
		status = "out of date"
	}
	fmt.Fprintf(stderr, "%s skill: installed (synced for v%s, CLI v%s) — %s\n",
		skills.Name(), strings.TrimPrefix(state.Version, "v"), strings.TrimPrefix(version, "v"), status)
	if !inSync {
		fmt.Fprintln(stderr, "Update it with: byted-supabase-cli skills install")
	}
	return nil
}

func writeJSON(w io.Writer, payload map[string]any) error {
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
