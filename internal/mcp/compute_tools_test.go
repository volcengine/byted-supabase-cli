// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeToolSpecs verifies that the compute group exposes the expected tools with
// the correct feature and mutating flags, without relying on global registration or a real server.
func TestComputeToolSpecs(t *testing.T) {
	specs := computeTools()
	byName := make(map[string]meta, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s.meta
	}

	require.Len(t, specs, 3)

	get, ok := byName["get_compute_settings"]
	require.True(t, ok, "expected get_compute_settings to be defined")
	assert.Equal(t, featureCompute, get.feature)
	assert.False(t, get.mutating, "get_compute_settings must be a read tool")

	modify, ok := byName["compute_settings"]
	require.True(t, ok, "expected modify_compute_settings to be defined")
	assert.Equal(t, featureCompute, modify.feature)
	assert.True(t, modify.mutating, "modify_compute_settings must be mutating")
	restart, ok := byName["restart_compute"]
	require.True(t, ok, "expected restart_compute to be defined")
	assert.Equal(t, featureCompute, restart.feature)
	assert.True(t, restart.mutating)
}

func TestRestartComputeRequiresBranchID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	_, err := restartBranch(context.Background(), &p, restartBranchInput{BranchID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch_id is required")
}

func TestComputeAndWorkspaceLifecycleDescriptionsKeepSleepAndStopSeparate(t *testing.T) {
	computeDescription := ""
	for _, spec := range computeTools() {
		if spec.meta.name == "compute_settings" {
			computeDescription = spec.meta.description
		}
	}
	require.NotEmpty(t, computeDescription)
	assert.Contains(t, computeDescription, "ModifyComputeSettings")
	assert.Contains(t, computeDescription, "suspend_timeout_seconds")
	assert.Contains(t, computeDescription, "does NOT immediately suspend")
	assert.Contains(t, computeDescription, "suspend_workspace")

	workspaceDescription := ""
	for _, spec := range accountTools() {
		if spec.meta.name == "stop_workspace" {
			workspaceDescription = spec.meta.description
		}
	}
	require.NotEmpty(t, workspaceDescription)
	assert.Contains(t, workspaceDescription, "StopWorkspace")
	assert.Contains(t, workspaceDescription, "休眠")
	assert.Contains(t, workspaceDescription, "NOT compute auto-suspend")
	suspendDescription := ""
	for _, spec := range accountTools() {
		if spec.meta.name == "sleep_workspace" {
			suspendDescription = spec.meta.description
		}
	}
	require.NotEmpty(t, suspendDescription)
	assert.Contains(t, suspendDescription, "SuspendWorkspace")
	assert.Contains(t, suspendDescription, "休眠")
	assert.NotContains(t, suspendDescription, "StopWorkspace")
	wakeDescription := ""
	for _, spec := range accountTools() {
		if spec.meta.name == "wake_workspace" {
			wakeDescription = spec.meta.description
		}
	}
	require.NotEmpty(t, wakeDescription)
	assert.Contains(t, wakeDescription, "DescribeWorkspaceEndpoint")
	assert.Contains(t, wakeDescription, "AddressDomain")
	assert.Contains(t, wakeDescription, "HTTP")
	assert.Contains(t, wakeDescription, "start_workspace")
}

// TestModifyComputeSettingsRequiresComputeIDForComputeOps verifies that compute-level
// operations (rename / resize) still require compute_id; an empty id is rejected before
// any network call.
func TestModifyComputeSettingsRequiresComputeIDForComputeOps(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	_, err := modifyComputeSettings(context.Background(), &p, modifyComputeSettingsInput{
		ComputeID:   "  ",
		ComputeName: "renamed",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compute_id is required to rename or resize")
}

// TestModifyComputeSettingsSuspendOnlyNeedsNoComputeID verifies that setting only the
// suspend timeout (a workspace-level operation) does not require compute_id — an empty
// compute_id must not trigger a parameter validation error (the call fails later at the
// config/network layer due to missing credentials).
func TestModifyComputeSettingsSuspendOnlyNeedsNoComputeID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	timeout := 1800
	_, err := modifyComputeSettings(context.Background(), &p, modifyComputeSettingsInput{
		WorkspaceID:           "ws-123",
		SuspendTimeoutSeconds: &timeout,
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "compute_id is required")
	assert.NotContains(t, err.Error(), "nothing to modify")
}

// TestModifyComputeSettingsRequiresChange rejects a no-op request (neither CU nor name changed) before any API call.
func TestModifyComputeSettingsRequiresChange(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	_, err := modifyComputeSettings(context.Background(), &p, modifyComputeSettingsInput{
		ComputeID: "cmp-123",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to modify")
}

// TestModifyComputeSettingsRequiresBothCU rejects a partial resize that supplies only one of min_cu/max_cu before any API call.
func TestModifyComputeSettingsRequiresBothCU(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	min := 0.5
	_, err := modifyComputeSettings(context.Background(), &p, modifyComputeSettingsInput{
		ComputeID: "cmp-123",
		MinCU:     &min,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "min_cu and max_cu are required together")
}

// TestModifyComputeSettingsAcceptsSuspendOnly verifies that providing only
// suspend_timeout_seconds is a valid change shape: it must not be blocked by the
// "no-op" or "missing CU" validations (the call will fail later at the config/network
// layer due to missing credentials, but that is not a parameter validation error).
func TestModifyComputeSettingsAcceptsSuspendOnly(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	zero := 0
	_, err := modifyComputeSettings(context.Background(), &p, modifyComputeSettingsInput{
		ComputeID:             "cmp-123",
		WorkspaceID:           "ws-123",
		SuspendTimeoutSeconds: &zero,
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "nothing to modify")
	assert.NotContains(t, err.Error(), "min_cu and max_cu are required together")
}

// TestModifyComputeSettingsRejectsInvalidSuspend verifies that an invalid suspend timeout (in
// the 1..299 range) is rejected before any API call, returning a clear error rather than a
// bare InvalidParameter from the backend.
func TestModifyComputeSettingsRejectsInvalidSuspend(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureCompute}})
	bad := 150
	_, err := modifyComputeSettings(context.Background(), &p, modifyComputeSettingsInput{
		WorkspaceID:           "ws-123",
		SuspendTimeoutSeconds: &bad,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid suspend_timeout_seconds")
}

// TestValidateSuspendTimeoutSeconds pins the suspend-timeout value semantics: only -1 (disable),
// 0 (unset), and [300, 604800] are valid.
func TestValidateSuspendTimeoutSeconds(t *testing.T) {
	valid := []int{-1, 0, 300, 1800, 604800}
	for _, v := range valid {
		assert.NoErrorf(t, validateSuspendTimeoutSeconds(v), "expected %d to be valid", v)
	}
	invalid := []int{-2, 1, 299, 604801}
	for _, v := range invalid {
		assert.Errorf(t, validateSuspendTimeoutSeconds(v), "expected %d to be rejected", v)
	}
}
