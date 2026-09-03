// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestPagesCommandGroup(t *testing.T) {
	group := pagesCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "pages", command.Name())
}

func TestPagesFactoryCommandTree(t *testing.T) {
	command := newPagesCommand(defaultRuntimeContext())
	for _, path := range [][]string{
		{"list"},
		{"env-vars"},
		{"binding"},
		{"bind"},
		{"create"},
		{"upload"},
		{"deploy"},
		{"deploy", "list"},
		{"fast", "create"},
		{"sync"},
		{"unbind"},
	} {
		found, _, err := command.Find(path)
		require.NoError(t, err)
		require.Equal(t, path[len(path)-1], found.Name())
	}
}

func TestPagesFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newPagesCommand(ctx)
	second := newPagesCommand(ctx)

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
