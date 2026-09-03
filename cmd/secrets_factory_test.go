// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestSecretsCommandGroup(t *testing.T) {
	group := secretsCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "secrets", command.Name())
}

func TestSecretsFactoryCommandTree(t *testing.T) {
	command := newSecretsCommand(defaultRuntimeContext())
	for _, name := range []string{"list", "set", "unset"} {
		found, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, found.Name())
	}

	setCommand, _, err := command.Find([]string{"set"})
	require.NoError(t, err)
	require.NotNil(t, setCommand.Flag("env-file"))
	require.NotNil(t, setCommand.Flag("branch-id"))
}

func TestSecretsFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newSecretsCommand(ctx)
	second := newSecretsCommand(ctx)

	require.NotSame(t, first, second)
	firstList, _, err := first.Find([]string{"list"})
	require.NoError(t, err)
	secondList, _, err := second.Find([]string{"list"})
	require.NoError(t, err)
	require.NotSame(t, firstList, secondList)

	require.NoError(t, firstList.Flags().Set("branch-id", "branch-first"))
	require.Equal(t, "branch-first", firstList.Flag("branch-id").Value.String())
	require.Equal(t, "", secondList.Flag("branch-id").Value.String())
}
