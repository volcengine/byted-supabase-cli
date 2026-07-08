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
)

const (
	// defaultSource is the `skills add` source collection that contains the
	// byted-supabase skill. defaultName selects that one skill out of the
	// collection (`-s byted-supabase`).
	defaultSource = "https://skills.volces.com/skills/bytedance/agentkit-samples"
	defaultName   = "byted-supabase"

	envSource      = "BYTED_SUPABASE_CLI_SKILLS_SOURCE"
	envName        = "BYTED_SUPABASE_CLI_SKILLS_NAME"
	envNoNotifier  = "BYTED_SUPABASE_CLI_NO_SKILLS_NOTIFIER"
	installTimeout = 2 * time.Minute
)

// Source returns the `skills add` source, overridable via env for testing or
// staging registries.
func Source() string {
	if v := os.Getenv(envSource); v != "" {
		return v
	}
	return defaultSource
}

// Name returns the skill name to install (`-s <name>`).
func Name() string {
	if v := os.Getenv(envName); v != "" {
		return v
	}
	return defaultName
}

// Runner installs the skill. The default implementation shells out to npx; tests
// inject a fake.
type Runner interface {
	Install(ctx context.Context, source, name string, force bool) *CmdResult
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

	cmd := opts.Runner.Install(ctx, Source(), name, opts.Force)
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
