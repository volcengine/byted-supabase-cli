// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/skills"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

type skillsOptions struct {
	Force bool
	JSON  bool
}

func skillsCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newSkillsCommand}}
}

func newSkillsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "skills", Short: skillsShortFactory(), Long: skillsLongFactory()}
	cmd.AddCommand(newSkillsInstallCommand(ctx), newSkillsStatusCommand(ctx))
	return cmd
}

func newSkillsInstallCommand(ctx RuntimeContext) *cobra.Command {
	var options skillsOptions
	cmd := &cobra.Command{
		Use: "install", Short: skillsInstallShortFactory(), Long: skillsInstallLongFactory(),
		Example: skillsInstallExampleFactory(), Aliases: []string{"update", "sync"}, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillsInstallFactory(cmd.Context(), options.Force, options.JSON, ctx.Output, ctx.Stderr)
		},
	}
	cmd.Flags().BoolVar(&options.Force, "force", false, "Reinstall even if already in sync.")
	cmd.Flags().BoolVar(&options.JSON, "json", false, "Output structured JSON.")
	return cmd
}

func newSkillsStatusCommand(ctx RuntimeContext) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{Use: "status", Short: "Show the installed skill version and sync state", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return runSkillsStatusFactory(asJSON, ctx.Output, ctx.Stderr)
	}}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output structured JSON.")
	return cmd
}

func runSkillsInstallFactory(ctx context.Context, force, asJSON bool, stdout, stderr io.Writer) error {
	version := strings.TrimSpace(utils.Version)
	if !force && skills.IsSynced(version) {
		if asJSON {
			return writeJSONFactory(stdout, map[string]any{"ok": true, "action": "in_sync", "skill": skills.Name(), "version": version, "message": fmt.Sprintf("%s skill is already in sync", skills.Name())})
		}
		_, _ = fmt.Fprintf(stderr, "✓ %s skill is already in sync (use --force to reinstall)\n", skills.Name())
		return nil
	}
	if !asJSON {
		_, _ = fmt.Fprintf(stderr, "Installing %s skill from %s ...\n", skills.Name(), skills.Source())
	}
	res := skills.Sync(ctx, skills.SyncOptions{Version: version, Force: force})
	if res.Action == "failed" {
		msg := fmt.Sprintf("skill install failed: %s", res.Err)
		if asJSON {
			_ = writeJSONFactory(stdout, map[string]any{"ok": false, "skill": skills.Name(), "error": map[string]any{"message": msg, "detail": res.Detail}})
		} else if res.Detail != "" {
			_, _ = fmt.Fprintln(stderr, res.Detail)
		}
		return errors.New(msg)
	}
	if asJSON {
		env := map[string]any{"ok": true, "action": "synced", "skill": res.Skill, "version": res.Version, "message": fmt.Sprintf("%s skill installed", res.Skill)}
		if res.Err != nil {
			env["warning"] = res.Err.Error()
		}
		return writeJSONFactory(stdout, env)
	}
	if res.Err != nil {
		_, _ = fmt.Fprintf(stderr, "⚠ %s skill installed (state not saved: %v)\n", res.Skill, res.Err)
		return nil
	}
	_, _ = fmt.Fprintf(stderr, "✓ %s skill installed\n", res.Skill)
	return nil
}

func runSkillsStatusFactory(asJSON bool, stdout, stderr io.Writer) error {
	version := strings.TrimSpace(utils.Version)
	state, ok, err := skills.ReadState()
	if err != nil {
		return errors.Errorf("failed to read skills state: %w", err)
	}
	installed := ok && state.Skill != ""
	versionTracked := installed && state.Version != "" && version != ""
	inSync := versionTracked && skills.IsSynced(version)
	if asJSON {
		env := map[string]any{"ok": true, "skill": skills.Name(), "cli_version": version, "installed": installed, "in_sync": inSync, "version_tracked": versionTracked}
		if installed {
			env["skill_version"], env["source"], env["updated_at"] = state.Version, state.Source, state.UpdatedAt
		}
		return writeJSONFactory(stdout, env)
	}
	if !installed {
		_, _ = fmt.Fprintf(stderr, "%s skill: not installed\n", skills.Name())
		_, _ = fmt.Fprintln(stderr, "Install it with: "+skills.InstallCommand())
		return nil
	}
	if !versionTracked {
		_, _ = fmt.Fprintf(stderr, "%s skill: installed (version not tracked on this build)\n", skills.Name())
		return nil
	}
	status := "in sync"
	if !inSync {
		status = "out of date"
	}
	_, _ = fmt.Fprintf(stderr, "%s skill: installed (synced for v%s, CLI v%s) — %s\n", skills.Name(), strings.TrimPrefix(state.Version, "v"), strings.TrimPrefix(version, "v"), status)
	if !inSync {
		_, _ = fmt.Fprintln(stderr, "Update it with: "+skills.InstallCommand())
	}
	return nil
}

func writeJSONFactory(w io.Writer, payload map[string]any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func skillsShortFactory() string { return fmt.Sprintf("Manage the %s Agent Skill", skills.Name()) }
func skillsLongFactory() string {
	return fmt.Sprintf("Install and inspect the %s skill (for Claude Code and other agents).\n\nThe skill is installed globally via the external `skills` tool (requires Node.js / npx). It is also kept in sync automatically by `%s update`.", skills.Name(), skills.CLIName())
}
func skillsInstallShortFactory() string {
	return fmt.Sprintf("Install or update the %s skill", skills.Name())
}
func skillsInstallLongFactory() string {
	return fmt.Sprintf("Install or update the %s skill, pinning it to this CLI version.\n\nRuns: npx -y skills add <source> -s %s -g -y\n\nUse --force to reinstall even if it is already in sync.\nUse --json for structured output (for scripts and AI agents).", skills.Name(), skills.Name())
}
func skillsInstallExampleFactory() string {
	cli := skills.CLIName()
	return fmt.Sprintf("  %s skills install\n  %s skills install --force\n  %s skills install --json", cli, cli, cli)
}
