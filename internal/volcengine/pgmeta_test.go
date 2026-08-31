// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectDefaultBranchID(t *testing.T) {
	t.Run("picks the branch flagged as default", func(t *testing.T) {
		id := selectDefaultBranchID([]Branch{
			{BranchID: "br-child", Default: false},
			{BranchID: "br-main", Default: true},
		})
		assert.Equal(t, "br-main", id)
	})

	t.Run("ignores branch status", func(t *testing.T) {
		// The whole point of resolving through DescribeBranches: the control plane's
		// DescribeDefaultBranch rejects any status other than Ready/Suspended, so a
		// branch mid-wakeup used to fail the lookup before the query even ran.
		for _, status := range []string{"Activating", "Restarting", "Resetting", "Suspending", "Stopped"} {
			id := selectDefaultBranchID([]Branch{{BranchID: "br-main", Default: true, BranchStatus: status}})
			assert.Equal(t, "br-main", id, "status %s", status)
		}
	})

	t.Run("returns empty when no branch is default", func(t *testing.T) {
		assert.Empty(t, selectDefaultBranchID([]Branch{{BranchID: "br-child"}}))
		assert.Empty(t, selectDefaultBranchID(nil))
	})

	t.Run("skips a default entry without an id", func(t *testing.T) {
		id := selectDefaultBranchID([]Branch{
			{BranchID: "  ", Default: true},
			{BranchID: "br-main", Default: true},
		})
		assert.Equal(t, "br-main", id)
	})
}

func TestResolveDefaultBranchIDShortCircuits(t *testing.T) {
	// A nil aidap client panics on any API call, so reaching the control plane here
	// would fail the test rather than silently cost a request.
	client := &Client{}

	id, err := client.ResolveDefaultBranchID(context.Background(), "ws-1", "  br-explicit  ")
	require.NoError(t, err)
	assert.Equal(t, "br-explicit", id)
}
