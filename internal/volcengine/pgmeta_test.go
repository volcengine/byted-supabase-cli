// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectDefaultBranch(t *testing.T) {
	t.Run("picks the branch flagged as default", func(t *testing.T) {
		branch, ok := selectDefaultBranch([]Branch{
			{BranchID: "br-child", Default: false},
			{BranchID: "br-main", Default: true},
		})
		require.True(t, ok)
		assert.Equal(t, "br-main", branch.BranchID)
	})

	t.Run("ignores branch status", func(t *testing.T) {
		// The whole point of resolving through DescribeBranches: the control plane's
		// DescribeDefaultBranch rejects any status other than Ready/Suspended, so a
		// branch mid-wakeup used to fail the lookup before the query even ran.
		for _, status := range []string{"Activating", "Restarting", "Resetting", "Suspending", "Stopped"} {
			branch, ok := selectDefaultBranch([]Branch{{BranchID: "br-main", Default: true, BranchStatus: status}})
			require.True(t, ok, "status %s", status)
			assert.Equal(t, "br-main", branch.BranchID, "status %s", status)
		}
	})

	t.Run("returns empty when no branch is default", func(t *testing.T) {
		_, ok := selectDefaultBranch([]Branch{{BranchID: "br-child"}})
		assert.False(t, ok)
		_, ok = selectDefaultBranch(nil)
		assert.False(t, ok)
	})

	t.Run("skips a default entry without an id", func(t *testing.T) {
		branch, ok := selectDefaultBranch([]Branch{
			{BranchID: "  ", Default: true},
			{BranchID: "br-main", Default: true},
		})
		require.True(t, ok)
		assert.Equal(t, "br-main", branch.BranchID)
	})
}

func TestResolveDefaultBranchUsesDescribeBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DescribeBranches", r.URL.Query().Get("Action"))
		assert.Equal(t, http.MethodPost, r.Method)
		var request struct {
			Offset int `json:"Offset"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, 0, request.Offset)
		_, _ = w.Write([]byte(`{"Result":{"Total":1,"Branches":[{"BranchId":"br-main","BranchStatus":"Activating","Default":true}]}}`))
	}))
	defer server.Close()
	client := NewClient(Config{
		AccessKeyID:     "test key id",
		SecretAccessKey: "test secret",
		Region:          DefaultRegion,
		Endpoint:        server.URL,
	})
	branch, err := client.ResolveDefaultBranch(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Equal(t, "br-main", branch.BranchID)
	assert.Equal(t, "Activating", branch.BranchStatus)
}

func TestResolveDefaultBranchPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Offset int `json:"Offset"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		switch request.Offset {
		case 0:
			_, _ = w.Write([]byte(`{"Result":{"Total":2,"Branches":[{"BranchId":"br-child","Default":false}]}}`))
		case 1:
			_, _ = w.Write([]byte(`{"Result":{"Total":2,"Branches":[{"BranchId":"br-main","BranchStatus":"Restarting","Default":true}]}}`))
		default:
			t.Errorf("unexpected offset %d", request.Offset)
		}
	}))
	defer server.Close()
	client := NewClient(Config{
		AccessKeyID:     "test key id",
		SecretAccessKey: "test secret",
		Region:          DefaultRegion,
		Endpoint:        server.URL,
	})
	branch, err := client.ResolveDefaultBranch(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Equal(t, "br-main", branch.BranchID)
	assert.Equal(t, "Restarting", branch.BranchStatus)
}
