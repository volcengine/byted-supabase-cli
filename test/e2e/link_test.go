// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitLinkUnlink exercises the local-dev linking flow in its own project
// directory: init a config, link the test workspace, verify the persisted
// coordinates, unlink. `link` hard-requires env AK/SK (RequireAccessKeysEnv),
// so the test is skipped on profile-only runs.
func TestInitLinkUnlink(t *testing.T) {
	if !hasEnvAccessKeys() {
		t.Skip("link requires VOLCENGINE_ACCESS_KEY/SECRET_KEY env; skipping on profile-only auth")
	}
	ctx := context.Background()
	dir := t.TempDir()

	res := runCLIIn(ctx, dir, queryTimeout, "init")
	require.Zero(t, res.ExitCode, res.failureDetail("init"))
	require.FileExists(t, filepath.Join(dir, "supabase", "config.toml"))

	res = runCLIIn(ctx, dir, queryTimeout, "link", "--workspace-id", h.workspaceID)
	require.Zero(t, res.ExitCode, res.failureDetail("link"))

	// link persists the workspace coordinates under supabase/.temp.
	ref, err := os.ReadFile(filepath.Join(dir, "supabase", ".temp", "project-ref"))
	require.NoError(t, err)
	assert.Equal(t, h.workspaceID, string(ref))

	res = runCLIIn(ctx, dir, queryTimeout, "unlink")
	require.Zero(t, res.ExitCode, res.failureDetail("unlink"))
	assert.NoFileExists(t, filepath.Join(dir, "supabase", ".temp", "project-ref"))
}
