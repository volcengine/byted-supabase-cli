// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// listLimit normalises the count parameter of list tools to a backend page size:
// falls back to volcengine.DefaultListLimit (10) when the caller omits it (count<=0).
// Shared by all paginated list tools so that "no count → 10 per page" is enforced
// uniformly at the MCP layer (including DescribeBranches/ListWorkspaces whose
// underlying defaults are 100 and cannot be changed because CLI also uses them).
func listLimit(count int) int {
	if count <= 0 {
		return volcengine.DefaultListLimit
	}
	return count
}

// allSpecs is the full tool catalog before policy filtering. New feature groups are
// registered here as thin adapters over fork-specific Run functions and volcengine.Client
// (see database_tools.go).
//
// Feature groups excluded by the agent seam are dropped here, before any policy
// runs: their tools neither register nor validate as --disabled-tools names, so
// a distribution that cuts a command group (e.g. pages) leaves no trace of it
// in the MCP surface either.
func allSpecs() []toolSpec {
	var specs []toolSpec
	specs = append(specs, systemTools()...)
	specs = append(specs, databaseTools()...)
	specs = append(specs, accountTools()...)
	specs = append(specs, developmentTools()...)
	specs = append(specs, branchingTools()...)
	specs = append(specs, functionsTools()...)
	specs = append(specs, storageTools()...)
	specs = append(specs, computeTools()...)
	specs = append(specs, pagesTools()...)
	specs = append(specs, authTools()...)

	avail := availableFeatures()
	kept := specs[:0]
	for _, t := range specs {
		if t.meta.feature == "" || avail[t.meta.feature] {
			kept = append(kept, t)
		}
	}
	return kept
}

// registerTools registers the allowed tools with the server. All policy filtering
// (feature groups, read-only, workspace binding, denylist) is applied in one place here.
//
// Backward-compatible aliases are collected for every registered tool and installed as a
// single receiving middleware that rewrites old names to their canonical name on
// tools/call (see aliases.go). Disabling the canonical name (above) drops the whole tool
// including its aliases; disabling an alias name removes only that one alias route while
// the canonical tool stays available.
func registerTools(s *mcp.Server, p *policy) {
	aliases := make(map[string]string)
	for _, t := range allSpecs() {
		if !p.allows(t.meta.name, t.meta.feature, t.meta.mutating) {
			continue
		}
		t.register(s, p)
		for _, a := range t.meta.aliases {
			if p.disabledTools[a] {
				continue
			}
			aliases[a] = t.meta.name
		}
	}
	if len(aliases) > 0 {
		s.AddReceivingMiddleware(aliasRewriteMiddleware(aliases))
	}
}

// systemTools are transport-layer tools without a feature group; they are always exposed regardless of the enabled feature set (only the denylist can remove them).
func systemTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "health_check",
			title:       "Health check",
			description: "Return the server identity and active access policy. Use to verify the MCP connection is working.",
		}, healthCheck),
	}
}

// healthInput has no parameters.
type healthInput struct{}

func healthCheck(_ context.Context, p *policy, _ healthInput) (string, error) {
	return toJSON(map[string]any{
		"server":    serverName(),
		"version":   version(),
		"read_only": p.readOnly,
		"features":  p.enabledFeatures(),
	})
}
