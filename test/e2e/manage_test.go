// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reversible mutations on the ephemeral test workspace. Each test restores the
// state it found, so ordering with the read-only tests does not matter.

func TestRenameCycle(t *testing.T) {
	ctx := context.Background()
	before, err := fetchDetail(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, before.Name)

	renamed := before.Name + "-renamed"
	res := runCLI(ctx, queryTimeout, "projects", "rename", h.workspaceID, "--name", renamed, "--yes")
	require.Zero(t, res.ExitCode, res.failureDetail("projects rename"))
	// Restore the original name even if the verification below fails.
	defer func() {
		res := runCLI(ctx, queryTimeout, "projects", "rename", h.workspaceID, "--name", before.Name, "--yes")
		assert.Zero(t, res.ExitCode, res.failureDetail("projects rename (restore)"))
	}()

	after, err := fetchDetail(ctx)
	require.NoError(t, err)
	assert.Equal(t, renamed, after.Name)
}

func TestTagsCycle(t *testing.T) {
	ctx := context.Background()
	res := runCLI(ctx, queryTimeout,
		"projects", "create-tags", h.workspaceID, "--tag", "e2e=smoke", "--yes")
	require.Zero(t, res.ExitCode, res.failureDetail("projects create-tags"))

	res = runCLI(ctx, queryTimeout,
		"projects", "delete-tags", h.workspaceID, "--key", "e2e", "--yes")
	assert.Zero(t, res.ExitCode, res.failureDetail("projects delete-tags"))
}
