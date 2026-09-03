// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestSeedCommandGroup(t *testing.T) {
	group := seedCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "seed", command.Name())
}

func TestSeedFactoryCommandTree(t *testing.T) {
	command := newSeedCommand(defaultRuntimeContext())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.PersistentFlags().Lookup("workspace-id"))
	found, _, err := command.Find([]string{"buckets"})
	require.NoError(t, err)
	require.Equal(t, "buckets", found.Name())
	require.NotNil(t, found.Flag("branch-id"))
	require.NotNil(t, found.Flag("linked"))
	require.NotNil(t, found.Flag("project-ref"))
	require.NotNil(t, found.Flag("workspace-id"))
}

func TestSeedFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newSeedCommand(ctx)
	second := newSeedCommand(ctx)

	require.NotSame(t, first, second)
	firstBuckets, _, err := first.Find([]string{"buckets"})
	require.NoError(t, err)
	secondBuckets, _, err := second.Find([]string{"buckets"})
	require.NoError(t, err)
	require.NotSame(t, firstBuckets, secondBuckets)

	require.NoError(t, firstBuckets.Flags().Set("branch-id", "branch-first"))
	require.Equal(t, "branch-first", firstBuckets.Flag("branch-id").Value.String())
	require.Equal(t, "", secondBuckets.Flag("branch-id").Value.String())
	require.NoError(t, firstBuckets.Flags().Set("workspace-id", "workspace-first"))
	require.Equal(t, "workspace-first", firstBuckets.Flag("workspace-id").Value.String())
	require.Equal(t, "", secondBuckets.Flag("workspace-id").Value.String())
}
