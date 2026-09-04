// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestComputesFactoryCommandTree(t *testing.T) {
	command := newComputesCommand(defaultRuntimeContext())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.PersistentFlags().Lookup("workspace-id"))

	for _, name := range []string{"list", "get", "update", "restart"} {
		leaf, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, leaf.Flags().Lookup("project-ref"), "%s missing project-ref", leaf.CommandPath())
		require.NotNil(t, leaf.Flags().Lookup("workspace-id"), "%s missing workspace-id", leaf.CommandPath())
	}
	_, _, err := command.Find([]string{"settings"})
	require.Error(t, err, "settings must be a projects command")
}

func TestComputesFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newComputesCommand(ctx)
	second := newComputesCommand(ctx)

	firstList, _, err := first.Find([]string{"list"})
	require.NoError(t, err)
	secondList, _, err := second.Find([]string{"list"})
	require.NoError(t, err)

	require.NotSame(t, firstList, secondList)
	require.NoError(t, firstList.Flags().Set("workspace-id", "workspace-first"))
	require.Equal(t, "workspace-first", firstList.Flag("workspace-id").Value.String())
	require.Equal(t, "", secondList.Flag("workspace-id").Value.String())
}
