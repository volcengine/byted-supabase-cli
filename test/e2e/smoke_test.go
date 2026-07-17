// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The workspace is provisioned once by TestMain and shared read-only by every
// test here, mirroring the upstream live-setup provide/inject pattern.

func TestWorkspaceDetail(t *testing.T) {
	detail, err := fetchDetail(context.Background())
	require.NoError(t, err)

	assert.Equal(t, h.workspaceID, detail.ReferenceID)
	assert.Equal(t, statusRunning, detail.Status)
	assert.NotEmpty(t, detail.Region)
}

func TestAPIKeys(t *testing.T) {
	res := runCLI(context.Background(), queryTimeout,
		"projects", "api-keys", "--workspace-id", h.workspaceID, "--output", "json")
	require.Zero(t, res.ExitCode, res.failureDetail("projects api-keys"))

	var keys []apiKey
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &keys), "stdout:\n%s", res.Stdout)

	// Match by type, not name: the control plane names them AnonKey /
	// ServiceRoleKey, but Public/Service is the stable classification.
	byType := make(map[string]apiKey, len(keys))
	for _, k := range keys {
		byType[k.Type] = k
	}
	anon, ok := byType["Public"]
	require.True(t, ok, "no Public (anon) key in %v", keys)
	serviceRole, ok := byType["Service"]
	require.True(t, ok, "no Service (service_role) key in %v", keys)

	assert.NotEmpty(t, anon.Key)
	assert.NotEmpty(t, serviceRole.Key)
	assert.NotEqual(t, anon.Key, serviceRole.Key)
}

func TestOperationsListable(t *testing.T) {
	// The create operation from setup must be visible in the audit log; assert
	// the command succeeds and returns parseable JSON without pinning its shape.
	res := runCLI(context.Background(), queryTimeout,
		"projects", "operations", h.workspaceID, "--output", "json")
	require.Zero(t, res.ExitCode, res.failureDetail("projects operations"))
	assert.True(t, json.Valid([]byte(res.Stdout)), "stdout is not valid JSON:\n%s", res.Stdout)
}
