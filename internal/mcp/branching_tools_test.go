// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchingToolsMeta(t *testing.T) {
	specs := branchingTools()

	byName := make(map[string]meta, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s.meta
	}
	require.Len(t, specs, 6)

	want := map[string]bool{ // name -> mutating
		"list_branches":      false,
		"get_branch":         false,
		"get_default_branch": false,
		"create_branch":      true,
		"delete_branch":      true,
		"restore_branch":     true,
	}
	for name, mutating := range want {
		m, ok := byName[name]
		require.True(t, ok, "expected tool %s to be defined", name)
		assert.Equal(t, featureBranching, m.feature, "tool %s should belong to the branching feature", name)
		assert.Equal(t, mutating, m.mutating, "tool %s mutating flag mismatch", name)
	}
}

func TestGetBranchRequiresBranchID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureBranching}})
	_, err := getBranch(context.Background(), &p, getBranchInput{BranchID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch_id is required")
}

func TestDeleteBranchRequiresBranchID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureBranching}})
	_, err := deleteBranch(context.Background(), &p, deleteBranchInput{BranchID: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch_id is required")
}

func TestRestoreBranchRequiresBranchID(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureBranching}})
	_, err := restoreBranch(context.Background(), &p, restoreBranchInput{BranchID: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "branch_id is required")
}

func TestRestoreBranchRequiresTime(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureBranching}})
	// Providing source but not time causes the backend to return InvalidParameter; intercept early for a clear error.
	_, err := restoreBranch(context.Background(), &p, restoreBranchInput{
		BranchID:       "br-1",
		SourceBranchID: "br-src",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "time is required")
}

func TestRestoreBranchRejectsInvalidTime(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureBranching}})
	_, err := restoreBranch(context.Background(), &p, restoreBranchInput{
		BranchID: "br-1",
		Time:     "2026-05-20 10:00:00",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid time")
}
