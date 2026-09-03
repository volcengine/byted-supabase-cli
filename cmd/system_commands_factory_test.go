// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestSkillsCommandGroup(t *testing.T) {
	group := skillsCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "skills", command.Name())
}

func TestSystemCommandFactories(t *testing.T) {
	root := newTestSystemCommandRoot()
	root.AddCommand(&cobra.Command{Use: "example"})
	installSystemCommandFactories(root)

	require.NotNil(t, commandByName(root, "help"))
	require.NotNil(t, commandByName(root, "completion"))
}

func TestSkillsAndUpdateFactoryTrees(t *testing.T) {
	skillsCommand := newSkillsCommand(defaultRuntimeContext())
	for _, name := range []string{"install", "status"} {
		command, _, err := skillsCommand.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, command.Name())
	}

	updateCommand := newUpdateCommand(defaultRuntimeContext())
	require.NotNil(t, updateCommand.Flag("check"))
	require.NotNil(t, updateCommand.Flag("force"))
	require.NotNil(t, updateCommand.Flag("json"))
}

func TestSkillsAndUpdateFactoriesCreateIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	firstSkills := newSkillsCommand(ctx)
	secondSkills := newSkillsCommand(ctx)
	firstInstall, _, err := firstSkills.Find([]string{"install"})
	require.NoError(t, err)
	secondInstall, _, err := secondSkills.Find([]string{"install"})
	require.NoError(t, err)
	require.NotSame(t, firstInstall, secondInstall)
	require.NoError(t, firstInstall.Flags().Set("force", "true"))
	require.Equal(t, "true", firstInstall.Flag("force").Value.String())
	require.Equal(t, "false", secondInstall.Flag("force").Value.String())
}

func newTestSystemCommandRoot() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

func commandByName(root *cobra.Command, name string) *cobra.Command {
	for _, command := range root.Commands() {
		if command.Name() == name {
			return command
		}
	}
	return nil
}
