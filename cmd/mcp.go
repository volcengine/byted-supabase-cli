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

var (
	mcpReadOnly        bool
	mcpFeatures        []string
	mcpDisabledTools   []string
	mcpWorkspaceRef    string
	mcpAgentPlan       bool
	mcpAgentPlanSeatID string

	mcpCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "mcp",
		Short:   "Model Context Protocol server",
	}

	mcpServeCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server over stdio",
		Long: `Start a Model Context Protocol server that exposes Byted Supabase CLI
capabilities as MCP tools over stdio.

An MCP client (Claude, Cursor, Trae, ...) spawns this command and communicates
over stdin/stdout. stdout carries the JSON-RPC stream only; logs go to stderr.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Flags take precedence; fall back to env vars when not explicitly set (mirrors the legacy Python server).
			f := cmd.Flags()
			return mcp.Serve(cmd.Context(), mcp.Options{
				ReadOnly:        resolveBoolFlag(f.Changed("read-only"), mcpReadOnly, os.Getenv("READ_ONLY")),
				Features:        resolveSliceFlag(f.Changed("features"), mcpFeatures, os.Getenv("FEATURES")),
				DisabledTools:   resolveSliceFlag(f.Changed("disabled-tools"), mcpDisabledTools, os.Getenv("DISABLED_TOOLS")),
				WorkspaceRef:    resolveStringFlag(f.Changed("workspace-ref"), mcpWorkspaceRef, os.Getenv("WORKSPACE_REF")),
				AgentPlan:       resolveBoolFlag(f.Changed("agent-plan"), mcpAgentPlan, os.Getenv("AGENT_PLAN")),
				AgentPlanSeatID: resolveStringFlag(f.Changed("agent-plan-seat-id"), mcpAgentPlanSeatID, os.Getenv("AGENT_PLAN_SEAT_ID")),
				Debug:           viper.GetBool("DEBUG"),
			})
		},
	}
)

func init() {
	serveFlags := mcpServeCmd.Flags()
	serveFlags.BoolVar(&mcpReadOnly, "read-only", false, "expose only read-only tools (env READ_ONLY)")
	serveFlags.StringSliceVar(&mcpFeatures, "features", nil, "comma-separated feature groups to enable; replaces (not extends) the default (default: all groups; env FEATURES)")
	serveFlags.StringSliceVar(&mcpDisabledTools, "disabled-tools", nil, "comma-separated tool names to disable (env DISABLED_TOOLS)")
	serveFlags.StringVar(&mcpWorkspaceRef, "workspace-ref", "", "hard-scope the server to a single workspace (env WORKSPACE_REF)")
	serveFlags.BoolVar(&mcpAgentPlan, "agent-plan", false, "default new workspaces to personal Agent Plan instances unless the create call says otherwise (env AGENT_PLAN)")
	serveFlags.StringVar(&mcpAgentPlanSeatID, "agent-plan-seat-id", "", "default new workspaces to enterprise Agent Plan instances bound to this seat id unless the create call says otherwise; takes precedence over --agent-plan (env AGENT_PLAN_SEAT_ID)")

	mcpCmd.AddCommand(mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)
}

// resolveBoolFlag returns the flag value when explicitly set, otherwise parses the env var.
func resolveBoolFlag(changed, flagVal bool, envVal string) bool {
	if changed {
		return flagVal
	}
	return parseBoolEnv(envVal)
}

// resolveStringFlag returns the flag value when explicitly set, otherwise returns the env var.
func resolveStringFlag(changed bool, flagVal, envVal string) string {
	if changed {
		return flagVal
	}
	return strings.TrimSpace(envVal)
}

// resolveSliceFlag returns the flag value when explicitly set, otherwise splits the env var on commas.
func resolveSliceFlag(changed bool, flagVal []string, envVal string) []string {
	if changed {
		return flagVal
	}
	return splitCSV(envVal)
}

// parseBoolEnv parses an env var as a boolean: 1/true/yes/on is true, anything else is false.
func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// splitCSV splits a string on commas; returns nil for empty input (trimming and deduplication are handled by the policy).
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, ",")
}
