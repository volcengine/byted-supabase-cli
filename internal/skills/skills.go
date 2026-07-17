// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Package skills installs and keeps the byted-supabase Claude/agent skill in
// sync with the running CLI binary. The skill itself is NOT bundled in this
// repo; it is published to skills.volces.com and installed globally via the
// external `skills` tool (`npx -y skills add <source> -s <name> -g -y`).
//
// Like the MCP layer, every file here is net-new so it never conflicts on an
// upstream rebase. The two touchpoints are:
//
//   - internal/update calls Sync after a CLI update so the skill tracks the
//     binary version (and installs it on first run).
//   - cmd/root.go calls Init at startup; on version drift it stores a pending
//     StaleNotice that Execute prints to stderr, mirroring the upgrade hint.
package skills

import (
	"context"
	"os"
	"time"

	"github.com/volcengine/byted-supabase-cli/agent"
)

const (
	// defaultSource is the `skills add` source collection that contains the
	// byted-supabase skill. defaultName selects that one skill out of the
	// collection (`-s byted-supabase`). defaultInstaller is the public `skills`
	// npm package run through npx. defaultRegistry is empty: the public tool
	// resolves from whatever registry the user's npm is configured with.
	defaultSource    = "https://skills.volces.com/skills/bytedance/agentkit-samples"
	defaultName      = "byted-supabase"
	defaultInstaller = "skills"
	defaultRegistry  = ""

	envSource      = "BYTED_SUPABASE_CLI_SKILLS_SOURCE"
	envName        = "BYTED_SUPABASE_CLI_SKILLS_NAME"
	envInstaller   = "BYTED_SUPABASE_CLI_SKILLS_INSTALLER"
	envRegistry    = "BYTED_SUPABASE_CLI_SKILLS_REGISTRY"
	envNoNotifier  = "BYTED_SUPABASE_CLI_NO_SKILLS_NOTIFIER"
	installTimeout = 2 * time.Minute
)

// Source returns the `skills add` source: the env override when set (testing
// or staging registries), else the registered agent seam's source (downstream
// distributions ship their own skill), else the upstream default.
func Source() string {
	if v := os.Getenv(envSource); v != "" {
		return v
	}
	if a := agent.Get(); a != nil {
		if s := a.Skill().Source; s != "" {
			return s
		}
	}
	return defaultSource
}

// Name returns the skill name to install (`-s <name>`), with the same
// precedence as Source: env, then agent seam, then upstream default.
func Name() string {
	if v := os.Getenv(envName); v != "" {
		return v
	}
	if a := agent.Get(); a != nil {
		if n := a.Skill().Name; n != "" {
			return n
		}
	}
	return defaultName
}

// Installer returns the npm package spec of the `skills` CLI to run through npx
// (e.g. "@example-scope/skills@latest" for a distribution's own channel), with
// the same precedence as Source/Name: env, then agent seam, then the public
// default.
func Installer() string {
	if v := os.Getenv(envInstaller); v != "" {
		return v
	}
	if a := agent.Get(); a != nil {
		if i := a.Skill().Installer; i != "" {
			return i
		}
	}
	return defaultInstaller
}

// Registry returns the npm registry to resolve Installer from (injected as
// npm_config_registry; empty keeps the user's npm configuration), with the same
// precedence as Source/Name: env, then agent seam, then the empty default.
func Registry() string {
	if v := os.Getenv(envRegistry); v != "" {
		return v
	}
	if a := agent.Get(); a != nil {
		if r := a.Skill().Registry; r != "" {
			return r
		}
	}
	return defaultRegistry
}

// InstallSpec bundles what to install and how to fetch the installer.
type InstallSpec struct {
	// Installer is the npm package spec of the `skills` CLI to run through npx.
	Installer string
	// Registry, when non-empty, pins npm_config_registry for the subprocess so
	// Installer resolves from that registry only.
	Registry string
	// Source and Name are the `skills add <source> -s <name>` arguments.
	Source string
	Name   string
}

// Runner installs the skill. The default implementation shells out to npx; tests
// inject a fake.
type Runner interface {
	Install(ctx context.Context, spec InstallSpec, force bool) *CmdResult
}

// SyncOptions controls a Sync call.
type SyncOptions struct {
	Version string           // CLI version to stamp into state; "" is treated as dev (still installs)
	Force   bool             // pass --force to reinstall even if the skill is current
	Runner  Runner           // nil → default npx runner
	Now     func() time.Time // nil → time.Now
}

// SyncResult is the outcome of a Sync call.
type SyncResult struct {
	Action  string // "synced" | "failed"
	Skill   string
	Version string
	Err     error
	Detail  string // trailing combined output on failure (for JSON/debug)
}

// Sync installs (or reinstalls) the skill and, on success, records the version
// in the global state file. It always runs the installer — callers that want to
// skip a redundant install should gate on IsSynced first.
func Sync(ctx context.Context, opts SyncOptions) *SyncResult {
	if opts.Runner == nil {
		opts.Runner = npxRunner{}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	name := Name()
	res := &SyncResult{Skill: name, Version: opts.Version}

	cmd := opts.Runner.Install(ctx, InstallSpec{
		Installer: Installer(),
		Registry:  Registry(),
		Source:    Source(),
		Name:      name,
	}, opts.Force)
	if cmd == nil || cmd.Err != nil {
		res.Action = "failed"
		res.Detail = combinedTail(cmd)
		if cmd != nil && cmd.Err != nil {
			res.Err = cmd.Err
		} else {
			res.Err = errEmptyResult
		}
		return res
	}

	res.Action = "synced"
	state := State{
		Skill:     name,
		Source:    Source(),
		Version:   opts.Version,
		UpdatedAt: opts.Now().UTC().Format(time.RFC3339),
	}
	if err := WriteState(state); err != nil {
		// The skill is installed; only the bookkeeping failed. Surface it but
		// don't call the whole sync a failure — the next run just re-checks.
		res.Err = err
		res.Detail = "skill installed but state not written: " + err.Error()
	}
	return res
}

// IsSynced reports whether the recorded skill version matches version (both
// normalized). Used by the update path to avoid re-running npx every time.
func IsSynced(version string) bool {
	recorded, ok := ReadSyncedVersion()
	if !ok {
		return false
	}
	return normalizeVersion(recorded) == normalizeVersion(version)
}
