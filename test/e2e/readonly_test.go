// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Read-only control-plane commands beyond the core smoke set. Shapes are not
// pinned unless the harness itself depends on them — these assert the command
// succeeds against a real workspace and emits parseable JSON.

func TestProjectsOverview(t *testing.T) {
	res := runCLI(context.Background(), queryTimeout, "projects", "overview", "--output", "json")
	require.Zero(t, res.ExitCode, res.failureDetail("projects overview"))
	assert.True(t, json.Valid([]byte(res.Stdout)), "stdout is not valid JSON:\n%s", res.Stdout)
}

func TestProjectsListContainsWorkspace(t *testing.T) {
	res := runCLI(context.Background(), queryTimeout, "projects", "list", "--output", "json")
	require.Zero(t, res.ExitCode, res.failureDetail("projects list"))

	var out struct {
		Projects []struct {
			ReferenceID string `json:"reference_id"`
			Status      string `json:"status"`
		} `json:"projects"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out), "stdout:\n%s", res.Stdout)
	for _, p := range out.Projects {
		if p.ReferenceID == h.workspaceID {
			assert.Equal(t, statusRunning, p.Status)
			return
		}
	}
	t.Fatalf("workspace %s not found in projects list (%d entries)", h.workspaceID, len(out.Projects))
}

func TestEndpointsList(t *testing.T) {
	res := runCLI(context.Background(), queryTimeout,
		"endpoints", "list", "--workspace-id", h.workspaceID, "--output", "json")
	require.Zero(t, res.ExitCode, res.failureDetail("endpoints list"))
	assert.True(t, json.Valid([]byte(res.Stdout)), "stdout is not valid JSON:\n%s", res.Stdout)
}

func TestBranchesList(t *testing.T) {
	// Every workspace is born with a default branch, so the list must parse
	// and be non-trivial.
	res := runCLI(context.Background(), queryTimeout,
		"branches", "list", "--workspace-id", h.workspaceID, "--output", "json")
	require.Zero(t, res.ExitCode, res.failureDetail("branches list"))
	assert.True(t, json.Valid([]byte(res.Stdout)), "stdout is not valid JSON:\n%s", res.Stdout)
	assert.NotEqual(t, "", strings.TrimSpace(res.Stdout))
}

func TestVersionFlag(t *testing.T) {
	res := runCLI(context.Background(), queryTimeout, "--version")
	require.Zero(t, res.ExitCode, res.failureDetail("--version"))
	assert.NotEmpty(t, strings.TrimSpace(res.Stdout))
}

func TestUpdateCheckJSON(t *testing.T) {
	// Exercises the self-update check path (npm registry, no Volcengine auth).
	res := runCLI(context.Background(), queryTimeout, "update", "--check", "--json")
	require.Zero(t, res.ExitCode, res.failureDetail("update --check --json"))

	var envelope struct {
		OK     bool   `json:"ok"`
		Action string `json:"action"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &envelope), "stdout:\n%s", res.Stdout)
	assert.True(t, envelope.OK)
	assert.Contains(t, []string{"update_available", "already_up_to_date"}, envelope.Action)
}
