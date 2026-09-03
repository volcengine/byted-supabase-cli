// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestNetworkRestrictionsCommandGroup(t *testing.T) {
	group := networkRestrictionsCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "network-restrictions", command.Name())
}

func TestNetworkRestrictionsFactoryCommandTree(t *testing.T) {
	command := newNetworkRestrictionsCommand(defaultRuntimeContext())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.PersistentFlags().Lookup("workspace-id"))
	for _, name := range []string{"get", "create", "update", "delete"} {
		found, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, found.Name())
		require.NotNil(t, found.Flags().Lookup("project-ref"), "%s missing project-ref", found.CommandPath())
		require.NotNil(t, found.Flags().Lookup("workspace-id"), "%s missing workspace-id", found.CommandPath())
	}
}

func TestNetworkRestrictionsFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newNetworkRestrictionsCommand(ctx)
	second := newNetworkRestrictionsCommand(ctx)

	require.NotSame(t, first, second)
	firstUpdate, _, err := first.Find([]string{"update"})
	require.NoError(t, err)
	secondUpdate, _, err := second.Find([]string{"update"})
	require.NoError(t, err)
	require.NotSame(t, firstUpdate, secondUpdate)

	require.NoError(t, firstUpdate.Flags().Set("ip", "10.0.0.1"))
	require.Equal(t, "[10.0.0.1]", firstUpdate.Flag("ip").Value.String())
	require.Equal(t, "[]", secondUpdate.Flag("ip").Value.String())
	require.NoError(t, firstUpdate.Flags().Set("workspace-id", "workspace-first"))
	require.Equal(t, "workspace-first", firstUpdate.Flag("workspace-id").Value.String())
	require.Equal(t, "", secondUpdate.Flag("workspace-id").Value.String())
}
