// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestProjectsCommandGroup(t *testing.T) {
	group := projectsCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "projects", command.Name())
}

func TestProjectsFactoryCommandTree(t *testing.T) {
	command := newProjectsCommand(defaultRuntimeContext())
	for _, name := range []string{
		"create",
		"delete",
		"list",
		"overview",
		"api-keys",
		"operations",
		"start",
		"stop",
		"rename",
		"deletion-protection",
		"create-tags",
		"delete-tags",
		"compute-settings",
		"workspace-settings",
	} {
		found, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, found.Name())
	}
}

func TestProjectsFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newProjectsCommand(ctx)
	second := newProjectsCommand(ctx)

	require.NotSame(t, first, second)
	firstRename, _, err := first.Find([]string{"rename"})
	require.NoError(t, err)
	secondRename, _, err := second.Find([]string{"rename"})
	require.NoError(t, err)
	require.NotSame(t, firstRename, secondRename)

	require.NoError(t, firstRename.Flags().Set("name", "first"))
	require.Equal(t, "first", firstRename.Flag("name").Value.String())
	require.Equal(t, "", secondRename.Flag("name").Value.String())
}
