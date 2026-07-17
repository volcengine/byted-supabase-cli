// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var errEmptyResult = errors.New("skills install produced no result")

// CmdResult captures the output of a skill install invocation.
type CmdResult struct {
	Stdout bytes.Buffer
	Stderr bytes.Buffer
	Err    error
}

func (r *CmdResult) combined() string { return r.Stdout.String() + r.Stderr.String() }

// npxRunner installs the skill via an external `skills`-style CLI run through npx:
//
//	npx -y <installer> add <source> -s <name> -g -y [--force]
//
// installer is the npm package: the public "skills" tool upstream, or a
// distribution's own (e.g. "@example-scope/skills@latest"). -g installs
// globally (skills live under the user's home, not the project), the leading
// -y auto-confirms npx's package fetch, and the trailing -y auto-confirms the
// skills tool's prompts. A non-empty spec.Registry is injected as
// npm_config_registry so the installer package resolves from that registry
// regardless of the user's npm configuration — required when the installer
// lives on a private registry, otherwise npx would fall through to the public
// one and could execute a same-named squatter package (dependency confusion).
type npxRunner struct{}

func (npxRunner) Install(ctx context.Context, spec InstallSpec, force bool) *CmdResult {
	r := &CmdResult{}
	npx, err := exec.LookPath("npx")
	if err != nil {
		r.Err = fmt.Errorf("npx not found in PATH (install Node.js to manage skills): %w", err)
		return r
	}
	installer := strings.TrimSpace(spec.Installer)
	if installer == "" {
		installer = "skills"
	}

	args := []string{"-y", installer, "add", spec.Source, "-s", spec.Name, "-g", "-y"}
	if force {
		args = append(args, "--force")
	}

	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, npx, args...)
	if reg := strings.TrimSpace(spec.Registry); reg != "" {
		cmd.Env = append(os.Environ(), "npm_config_registry="+reg)
	}
	cmd.Stdout = &r.Stdout
	cmd.Stderr = &r.Stderr
	r.Err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		r.Err = fmt.Errorf("skills install timed out after %s", installTimeout)
	}
	return r
}

// combinedTail returns the trailing portion of combined output, for embedding in
// error envelopes without dumping a full npm log.
func combinedTail(r *CmdResult) string {
	if r == nil {
		return ""
	}
	const maxLen = 1000
	out := strings.TrimSpace(r.combined())
	runes := []rune(out)
	if len(runes) <= maxLen {
		return out
	}
	return string(runes[len(runes)-maxLen:])
}
