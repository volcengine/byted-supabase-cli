// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Package mcp implements the `byted-supabase-cli mcp serve` command: a Model Context
// Protocol server that exposes the CLI's Volcengine Supabase (aidap) capabilities as
// MCP tools.
//
// Tools are thin adapters over the CLI's existing Run() functions and the shared
// volcengine.Client, keeping MCP and CLI commands in sync from a single source of
// truth rather than reimplementing AIDAP access as the legacy Python server did.
//
// Only stdio transport is implemented: an MCP client spawns this binary and exchanges
// newline-delimited JSON-RPC over stdin/stdout. stdout carries the protocol stream
// only; all logs go to stderr.
package mcp

// Options configures the MCP server.
//
// The access-policy fields mirror the configuration surface of the legacy Python
// server (FEATURES / READ_ONLY / DISABLED_TOOLS / WORKSPACE_REF) so the full tool
// set can be filled in under the same gating without changing the struct.
type Options struct {
	// ReadOnly hides all mutating tools, exposing only read-only tools.
	ReadOnly bool
	// Features is the set of feature groups to enable; empty means the default set.
	Features []string
	// DisabledTools is the denylist of tool names, applied after all other policy checks.
	DisabledTools []string
	// WorkspaceRef hard-scopes the server to a single workspace: the account group is
	// hidden and workspace-constrained tools are fixed to that target.
	WorkspaceRef string
	// AgentPlan force-creates new workspaces as personal Agent Plan instances. It is a
	// server-side default only: an explicit agent-plan choice in the create_workspace call
	// (including is_agent_plan=false) always wins. AgentPlanSeatID takes precedence over it.
	AgentPlan bool
	// AgentPlanSeatID force-creates new workspaces as enterprise Agent Plan instances bound
	// to this seat (implies agent plan). Server-side default only; explicit caller input wins.
	AgentPlanSeatID string
	// Debug raises the server log level to debug (output goes to stderr).
	Debug bool
}
