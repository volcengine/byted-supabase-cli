// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestMCPFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newMCPCommand(ctx)
	second := newMCPCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "mcp", first.Name())

	firstServe, _, err := first.Find([]string{"serve"})
	require.NoError(t, err)
	secondServe, _, err := second.Find([]string{"serve"})
	require.NoError(t, err)
	require.NotSame(t, firstServe, secondServe)
	require.NoError(t, firstServe.Flags().Set("workspace-ref", "workspace-first"))
	require.Equal(t, "workspace-first", firstServe.Flag("workspace-ref").Value.String())
	require.Equal(t, "", secondServe.Flag("workspace-ref").Value.String())
}

func TestMCPFactoryCommandTree(t *testing.T) {
	command := newMCPCommand(defaultRuntimeContext())
	require.Len(t, command.Commands(), 1)
	require.Equal(t, "serve", command.Commands()[0].Name())
}
