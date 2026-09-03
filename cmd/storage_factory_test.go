// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestStorageCommandGroup(t *testing.T) {
	group := storageCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "storage", command.Name())
}

func TestStorageFactoryCommandTree(t *testing.T) {
	command := newStorageCommand(defaultRuntimeContext())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.PersistentFlags().Lookup("workspace-id"))
	for _, path := range [][]string{
		{"ls"},
		{"cp"},
		{"mv"},
		{"rm"},
		{"buckets", "list"},
		{"buckets", "get"},
		{"buckets", "create"},
		{"buckets", "update"},
		{"buckets", "delete"},
	} {
		found, _, err := command.Find(path)
		require.NoError(t, err)
		require.Equal(t, path[len(path)-1], found.Name())
		require.NotNil(t, found.Flags().Lookup("project-ref"), "%s missing project-ref", found.CommandPath())
		require.NotNil(t, found.Flags().Lookup("workspace-id"), "%s missing workspace-id", found.CommandPath())
		require.NotNil(t, found.Flags().Lookup("linked"), "%s missing linked", found.CommandPath())
		require.NotNil(t, found.Flags().Lookup("branch-id"), "%s missing branch-id", found.CommandPath())
	}
}

func TestStorageFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newStorageCommand(ctx)
	second := newStorageCommand(ctx)

	require.NotSame(t, first, second)
	firstList, _, err := first.Find([]string{"ls"})
	require.NoError(t, err)
	secondList, _, err := second.Find([]string{"ls"})
	require.NoError(t, err)
	require.NotSame(t, firstList, secondList)

	require.NoError(t, firstList.Flags().Set("recursive", "true"))
	require.Equal(t, "true", firstList.Flag("recursive").Value.String())
	require.Equal(t, "false", secondList.Flag("recursive").Value.String())
	require.NoError(t, firstList.Flags().Set("workspace-id", "workspace-first"))
	require.Equal(t, "workspace-first", firstList.Flag("workspace-id").Value.String())
	require.Equal(t, "", secondList.Flag("workspace-id").Value.String())

	firstCreate, _, err := first.Find([]string{"buckets", "create"})
	require.NoError(t, err)
	secondCreate, _, err := second.Find([]string{"buckets", "create"})
	require.NoError(t, err)
	require.NoError(t, firstCreate.Flags().Set("public", "true"))
	require.Equal(t, "true", firstCreate.Flag("public").Value.String())
	require.Equal(t, "false", secondCreate.Flag("public").Value.String())
}
