// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestAuthFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newAuthCommand(ctx)
	second := newAuthCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupManagementAPI, first.GroupID)

	firstGet, _, err := first.Find([]string{"config", "get"})
	require.NoError(t, err)
	secondGet, _, err := second.Find([]string{"config", "get"})
	require.NoError(t, err)
	require.NotSame(t, firstGet, secondGet)

	require.NoError(t, firstGet.Flags().Set("branch-id", "branch-first"))
	require.Equal(t, "branch-first", firstGet.Flag("branch-id").Value.String())
	require.Equal(t, "", secondGet.Flag("branch-id").Value.String())
}

func TestAuthFactoryCommandTree(t *testing.T) {
	cmd := newAuthCommand(defaultRuntimeContext())
	for _, path := range [][]string{
		{"config"}, {"config", "get"}, {"config", "set"},
		{"hooks"}, {"hooks", "get"}, {"hooks", "set"},
		{"third-party"}, {"third-party", "list"}, {"third-party", "add"},
		{"third-party", "remove"}, {"third-party", "sync"},
	} {
		found, _, err := cmd.Find(path)
		require.NoError(t, err)
		require.Equal(t, path[len(path)-1], found.Name())
	}
}
