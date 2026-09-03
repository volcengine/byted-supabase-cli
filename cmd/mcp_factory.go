// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/mcp"
)

type mcpOptions struct {
	ReadOnly        bool
	Features        []string
	DisabledTools   []string
	WorkspaceRef    string
	AgentPlan       bool
	AgentPlanSeatID string
}

func newMCPCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "mcp",
		Short:   "Model Context Protocol Server",
	}
	cmd.AddCommand(newMCPServeCommand(ctx))
	return cmd
}

func newMCPServeCommand(ctx RuntimeContext) *cobra.Command {
	var options mcpOptions
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server over stdio",
		Long: `Start a Model Context Protocol server that exposes Byted Supabase CLI
capabilities as MCP tools over stdio.

An MCP client (Claude, Cursor, Trae, ...) spawns this command and communicates
over stdin/stdout. stdout carries the JSON-RPC stream only; logs go to stderr.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			return mcp.Serve(cmd.Context(), mcp.Options{
				ReadOnly:        resolveBoolFlag(flags.Changed("read-only"), options.ReadOnly, os.Getenv("READ_ONLY")),
				Features:        resolveSliceFlag(flags.Changed("features"), options.Features, os.Getenv("FEATURES")),
				DisabledTools:   resolveSliceFlag(flags.Changed("disabled-tools"), options.DisabledTools, os.Getenv("DISABLED_TOOLS")),
				WorkspaceRef:    resolveStringFlag(flags.Changed("workspace-ref"), options.WorkspaceRef, os.Getenv("WORKSPACE_REF")),
				AgentPlan:       resolveBoolFlag(flags.Changed("agent-plan"), options.AgentPlan, os.Getenv("AGENT_PLAN")),
				AgentPlanSeatID: resolveStringFlag(flags.Changed("agent-plan-seat-id"), options.AgentPlanSeatID, os.Getenv("AGENT_PLAN_SEAT_ID")),
				Debug:           viper.GetBool("DEBUG"),
			})
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&options.ReadOnly, "read-only", false, "expose only read-only tools (env READ_ONLY)")
	flags.StringSliceVar(&options.Features, "features", nil, "comma-separated feature groups to enable; replaces (not extends) the default (default: all groups; env FEATURES)")
	flags.StringSliceVar(&options.DisabledTools, "disabled-tools", nil, "comma-separated tool names to disable (env DISABLED_TOOLS)")
	flags.StringVar(&options.WorkspaceRef, "workspace-ref", "", "hard-scope the server to a single workspace (env WORKSPACE_REF)")
	flags.BoolVar(&options.AgentPlan, "agent-plan", false, "default new workspaces to personal Agent Plan instances unless the create call says otherwise (env AGENT_PLAN)")
	flags.StringVar(&options.AgentPlanSeatID, "agent-plan-seat-id", "", "default new workspaces to enterprise Agent Plan instances bound to this seat id unless the create call says otherwise; takes precedence over --agent-plan (env AGENT_PLAN_SEAT_ID)")
	_ = ctx
	return cmd
}

func resolveBoolFlag(changed, flagVal bool, envVal string) bool {
	if changed {
		return flagVal
	}
	return parseBoolEnv(envVal)
}

func resolveStringFlag(changed bool, flagVal, envVal string) string {
	if changed {
		return flagVal
	}
	return strings.TrimSpace(envVal)
}

func resolveSliceFlag(changed bool, flagVal []string, envVal string) []string {
	if changed {
		return flagVal
	}
	return splitCSV(envVal)
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}
