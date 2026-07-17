// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Data-plane commands go through the branch gateway, which needs a reachable
// endpoint — new workspaces have public access disabled, so this suite first
// enables it on the default branch. That network-exposure mutation is why the
// whole suite is opt-in via BYTED_SUPABASE_E2E_DATA_PLANE=1. Public access is
// disabled again at the end (best-effort).
func TestDataPlane(t *testing.T) {
	if os.Getenv(envDataPlane) == "" {
		t.Skipf("data-plane suite disabled; set %s=1 to run (enables public endpoint access on the test workspace)", envDataPlane)
	}
	ctx := context.Background()

	res := runCLI(ctx, queryTimeout, "endpoints", "enable-public", "--workspace-id", h.workspaceID)
	require.Zero(t, res.ExitCode, res.failureDetail("endpoints enable-public"))
	defer func() {
		res := runCLI(ctx, queryTimeout, "endpoints", "disable-public", "--workspace-id", h.workspaceID)
		if res.ExitCode != 0 {
			t.Logf("endpoints disable-public failed (workspace is deleted manually anyway): %s", res.Stderr)
		}
	}()

	// The gateway address takes a moment to become resolvable after
	// enable-public; wait through the cheapest data-plane reader.
	require.NoError(t, waitForDataPlane(ctx, 3*time.Minute))

	t.Run("SecretsList", func(t *testing.T) {
		res := runCLI(ctx, queryTimeout, "secrets", "list", "--workspace-id", h.workspaceID, "--output", "json")
		require.Zero(t, res.ExitCode, res.failureDetail("secrets list"))
		assert.True(t, json.Valid([]byte(res.Stdout)), "stdout is not valid JSON:\n%s", res.Stdout)
	})

	t.Run("FunctionsList", func(t *testing.T) {
		res := runCLI(ctx, queryTimeout, "functions", "list", "--workspace-id", h.workspaceID, "--output", "json")
		require.Zero(t, res.ExitCode, res.failureDetail("functions list"))
		assert.True(t, json.Valid([]byte(res.Stdout)), "stdout is not valid JSON:\n%s", res.Stdout)
	})

	t.Run("StorageList", func(t *testing.T) {
		res := runCLI(ctx, queryTimeout, "storage", "ls", "--workspace-id", h.workspaceID)
		require.Zero(t, res.ExitCode, res.failureDetail("storage ls"))
	})

	t.Run("DBQuery", func(t *testing.T) {
		res := runCLI(ctx, queryTimeout, "db", "query", "select 1 as one", "--workspace-id", h.workspaceID, "--agent=no", "--output", "json")
		require.Zero(t, res.ExitCode, res.failureDetail("db query"))
		assert.Contains(t, res.Stdout, "one")
	})

	t.Run("GenTypes", func(t *testing.T) {
		res := runCLI(ctx, queryTimeout, "gen", "types", "--workspace-id", h.workspaceID, "--lang", "typescript")
		require.Zero(t, res.ExitCode, res.failureDetail("gen types"))
		assert.NotEmpty(t, strings.TrimSpace(res.Stdout))
	})
}

// waitForDataPlane polls `secrets list` until the branch gateway answers, or
// the deadline passes. Any success means endpoint resolution works for every
// data-plane command, since they share the same access-resolution chain.
func waitForDataPlane(ctx context.Context, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	var last cliResult
	for time.Now().Before(deadline) {
		last = runCLI(ctx, queryTimeout, "secrets", "list", "--workspace-id", h.workspaceID, "--output", "json")
		if last.ExitCode == 0 {
			return nil
		}
		time.Sleep(readyPollInterval)
	}
	return fmt.Errorf("%s", last.failureDetail("secrets list (data-plane readiness)"))
}
