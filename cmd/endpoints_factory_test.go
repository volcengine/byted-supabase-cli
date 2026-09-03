// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestEndpointsCommandGroup(t *testing.T) {
	group := endpointsCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "endpoints", command.Name())
}

func TestEndpointsFactoryCommandTree(t *testing.T) {
	command := newEndpointsCommand(defaultRuntimeContext())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.PersistentFlags().Lookup("workspace-id"))
	for _, path := range [][]string{
		{"list"},
		{"enable-public"},
		{"disable-public"},
		{"enable-private"},
		{"eips", "list"},
		{"vpcs", "list"},
		{"subnets", "list"},
	} {
		found, _, err := command.Find(path)
		require.NoError(t, err)
		require.Equal(t, path[len(path)-1], found.Name())
	}

	for _, path := range [][]string{
		{"list"},
		{"enable-public"},
		{"disable-public"},
		{"enable-private"},
	} {
		found, _, err := command.Find(path)
		require.NoError(t, err)
		require.NotNil(t, found.Flags().Lookup("project-ref"), "%s missing project-ref", found.CommandPath())
		require.NotNil(t, found.Flags().Lookup("workspace-id"), "%s missing workspace-id", found.CommandPath())
	}
}

func TestEndpointsFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newEndpointsCommand(ctx)
	second := newEndpointsCommand(ctx)

	require.NotSame(t, first, second)
	firstList, _, err := first.Find([]string{"list"})
	require.NoError(t, err)
	secondList, _, err := second.Find([]string{"list"})
	require.NoError(t, err)
	require.NotSame(t, firstList, secondList)

	require.NoError(t, firstList.Flags().Set("branch-id", "branch-first"))
	require.Equal(t, "branch-first", firstList.Flag("branch-id").Value.String())
	require.Equal(t, "", secondList.Flag("branch-id").Value.String())
	require.NoError(t, firstList.Flags().Set("workspace-id", "workspace-first"))
	require.Equal(t, "workspace-first", firstList.Flag("workspace-id").Value.String())
	require.Equal(t, "", secondList.Flag("workspace-id").Value.String())
}
