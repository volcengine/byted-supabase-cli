// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestBranchesCommandGroup(t *testing.T) {
	group := branchesCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "branches", command.Name())
}

func TestBranchesFactoryCommandTree(t *testing.T) {
	command := newBranchesCommand(defaultRuntimeContext())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.PersistentFlags().Lookup("workspace-id"))
	for _, path := range [][]string{
		{"create"},
		{"list"},
		{"get"},
		{"update"},
		{"delete"},
		{"get-default"},
		{"set-default"},
		{"restart"},
		{"restore-window"},
		{"restorable"},
		{"restore"},
		{"update-studio-login"},
	} {
		found, _, err := command.Find(path)
		require.NoError(t, err)
		require.Equal(t, path[len(path)-1], found.Name())
		require.NotNil(t, found.Flags().Lookup("workspace-id"), "%s missing workspace-id", found.CommandPath())
		require.NotNil(t, found.Flags().Lookup("project-ref"), "%s missing project-ref", found.CommandPath())
	}
}

func TestBranchesFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newBranchesCommand(ctx)
	second := newBranchesCommand(ctx)

	require.NotSame(t, first, second)
	firstList, _, err := first.Find([]string{"list"})
	require.NoError(t, err)
	secondList, _, err := second.Find([]string{"list"})
	require.NoError(t, err)
	require.NotSame(t, firstList, secondList)

	require.NoError(t, firstList.Flags().Set("search", "first"))
	require.Equal(t, "first", firstList.Flag("search").Value.String())
	require.Equal(t, "", secondList.Flag("search").Value.String())
}
