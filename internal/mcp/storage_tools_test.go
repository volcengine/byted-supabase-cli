// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageToolsMeta(t *testing.T) {
	specs := storageTools()

	byName := make(map[string]meta, len(specs))
	for _, s := range specs {
		byName[s.meta.name] = s.meta
	}
	require.Len(t, specs, 4)

	want := map[string]bool{ // name -> mutating
		"list_storage_buckets":  false,
		"create_storage_bucket": true,
		"delete_storage_bucket": true,
		"get_storage_config":    false,
	}
	for name, mutating := range want {
		m, ok := byName[name]
		require.True(t, ok, "expected tool %s to be defined", name)
		assert.Equal(t, featureStorage, m.feature, "tool %s should belong to the storage feature", name)
		assert.Equal(t, mutating, m.mutating, "tool %s mutating flag mismatch", name)
	}
}

func TestCreateStorageBucketRequiresName(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureStorage}})
	_, err := createStorageBucket(context.Background(), &p, createStorageBucketInput{Name: "  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestDeleteStorageBucketRequiresName(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureStorage}})
	_, err := deleteStorageBucket(context.Background(), &p, deleteStorageBucketInput{Name: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}
