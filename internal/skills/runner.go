// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// npxRunner installs the skill via the external `skills` CLI run through npx:
//
//	npx -y skills add <source> -s <name> -g -y [--force]
//
// -g installs globally (skills live under the user's home, not the project),
// the leading -y auto-confirms npx's package fetch, and the trailing -y
// auto-confirms the skills tool's prompts.
type npxRunner struct{}

func (npxRunner) Install(ctx context.Context, source, name string, force bool) *CmdResult {
	r := &CmdResult{}
	npx, err := exec.LookPath("npx")
	if err != nil {
		r.Err = fmt.Errorf("npx not found in PATH (install Node.js to manage skills): %w", err)
		return r
	}

	args := []string{"-y", "skills", "add", source, "-s", name, "-g", "-y"}
	if force {
		args = append(args, "--force")
	}

	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, npx, args...)
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
