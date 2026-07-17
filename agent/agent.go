// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Package agent is the agent-surface seam for the CLI.
//
// The CLI ships two agent-facing subsystems that live under internal/ and are
// therefore invisible to downstream modules: the installable skill
// (internal/skills — fetched at runtime from a remote registry, never bundled
// in the binary) and the MCP server (internal/mcp — spawned by MCP clients
// via mcp.json, so a downstream cannot inject defaults at invocation time).
// A downstream distribution needs both to carry its own identity: its own
// skill source and name, its own MCP server identity, and its own feature cut
// (for example dropping Volcengine-only groups).
//
// This package inverts control the same way backend and distribution do: the
// downstream registers an Agent via Set during process startup, and
// internal/skills / internal/mcp consult it. The interface is deliberately
// thin — it injects configuration and policy, never replaces subsystem
// behaviour. The BYTED_SUPABASE_CLI_SKILLS_SOURCE/NAME environment overrides
// still take precedence over the registered agent, keeping the existing
// testing/staging escape hatch.
//
// Like backend and distribution, the dependency direction is strictly
// one-way: this open-source module never imports any distribution-specific
// package. Distributions depend on this package, not the other way around.
package agent

// Skill identifies the agent skill the CLI installs and keeps in sync via an
// external `skills`-style tool (`npx -y <installer> add <source> -s <name> -g -y`).
type Skill struct {
	// Source is the `skills add` source: a collection URL for the public tool,
	// or whatever source form the distribution's own installer understands
	// (e.g. a repo path like "example-org/skills-repo").
	Source string
	// Name selects one skill out of the source (`-s <name>`).
	Name string
	// Installer is the npm package spec of the `skills` CLI to run through npx
	// (e.g. "@example-scope/skills@latest"). Empty keeps the upstream public
	// "skills" tool. Both accept the same `add <source> -s <name> -g -y` shape.
	Installer string
	// Registry is the npm registry npx resolves Installer from, injected as
	// npm_config_registry into the install subprocess. Empty keeps the user's
	// npm configuration. Distributions whose Installer lives on a private
	// registry should set this: it pins resolution so a same-named package on
	// the public registry can never be fetched and executed instead
	// (dependency confusion).
	Registry string
}

// MCPServer is the identity the MCP server advertises to clients.
type MCPServer struct {
	// Name is the JSON-RPC implementation name; it is also echoed by the
	// health_check tool.
	Name string
	// Title is the human-readable server title.
	Title string
	// Instructions is the usage hint advertised to connecting clients.
	Instructions string
}

// Agent customizes the CLI's agent surface. Embed Base and override only the
// customizations the distribution needs.
type Agent interface {
	// Skill returns the skill to install. Empty fields keep the upstream
	// defaults; the skills env overrides win over both.
	Skill() Skill

	// MCPServer returns overrides for the MCP server identity. Empty fields
	// keep the upstream values.
	MCPServer() MCPServer

	// IncludeMCPFeature reports whether the MCP feature group (e.g. "pages")
	// exists in this build. Excluded groups drop out of the default feature
	// set, the --features / --disabled-tools validation, and tool
	// registration — mirroring distribution.IncludeCommand for the command
	// tree.
	IncludeMCPFeature(feature string) bool
}

// Base is a no-op Agent meant for embedding, so implementations only spell
// out what they customize.
type Base struct{}

func (Base) Skill() Skill                  { return Skill{} }
func (Base) MCPServer() MCPServer          { return MCPServer{} }
func (Base) IncludeMCPFeature(string) bool { return true }

var registered Agent

// Set installs the agent customization. Call it once during process startup,
// before any command runs. Passing nil restores upstream behaviour.
func Set(a Agent) { registered = a }

// Get returns the registered agent, or nil when none is installed.
func Get() Agent { return registered }
