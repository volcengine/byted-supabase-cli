// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/errdefs"
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
		"sleep",
		"wake",
		"settings",
		"create-tags",
		"delete-tags",
	} {
		found, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, found.Name())
	}
	for _, name := range []string{"start", "stop", "sleep", "wake"} {
		leaf, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Error(t, leaf.Args(leaf, []string{"positional-id"}), name+" must reject positional workspace ids")
		require.NotNil(t, leaf.Flags().Lookup("workspace-id"))
		require.NotNil(t, leaf.Flags().Lookup("project-ref"))
	}
}

func TestProjectsSettingsIsWorkspaceManagementCommand(t *testing.T) {
	command := newProjectsCommand(defaultRuntimeContext())
	settings, _, err := command.Find([]string{"settings"})
	require.NoError(t, err)
	require.Equal(t, "settings", settings.Name())
	compute, _, err := command.Find([]string{"settings", "compute"})
	require.NoError(t, err)
	require.Equal(t, "compute", compute.Name())
	require.Equal(t, "Configure workspace compute and auto-sleep settings", compute.Short)
	require.NotNil(t, compute.Flags().Lookup("min-cu"))
	require.NotNil(t, compute.Flags().Lookup("max-cu"))
	_, _, err = newComputesCommand(defaultRuntimeContext()).Find([]string{"settings"})
	require.Error(t, err)
	for _, name := range []string{"compute", "rename", "retention", "deletion-protection", "network", "private-dns"} {
		_, _, err = command.Find([]string{"settings", name})
		require.NoError(t, err, "missing projects settings subcommand %s", name)
	}
	computeCommand, _, err := command.Find([]string{"settings", "compute"})
	require.NoError(t, err)
	require.Error(t, computeCommand.Args(computeCommand, []string{"positional-workspace-id"}))
	for _, flag := range []string{"workspace-id", "project-ref", "min-cu", "max-cu"} {
		require.NotNil(t, computeCommand.Flags().Lookup(flag), "missing compute flag %s", flag)
	}
	nameCommand, _, err := command.Find([]string{"settings", "rename"})
	require.NoError(t, err)
	require.Contains(t, nameCommand.Example, "settings rename")
	require.NotNil(t, nameCommand.Flags().Lookup("workspace-id"))
	require.NotNil(t, nameCommand.Flags().Lookup("project-ref"))
}

func TestProjectsWorkspaceScopedCommandsUseWorkspaceFlags(t *testing.T) {
	command := newProjectsCommand(defaultRuntimeContext())
	workspaceCommands := [][]string{
		{"overview"},
		{"api-keys"},
		{"operations"},
		{"start"},
		{"stop"},
		{"sleep"},
		{"wake"},
		{"settings", "compute"},
		{"settings", "rename"},
		{"settings", "retention"},
		{"settings", "deletion-protection"},
		{"settings", "network"},
		{"settings", "private-dns"},
		{"create-tags"},
		{"delete-tags"},
	}
	for _, path := range workspaceCommands {
		found, _, err := command.Find(path)
		require.NoError(t, err, "missing projects command %v", path)
		require.NotNil(t, found.Flags().Lookup("workspace-id"), "%v missing --workspace-id", path)
		require.NotNil(t, found.Flags().Lookup("project-ref"), "%v missing --project-ref", path)
	}
}

func TestProjectsLifecycleUsesExplicitWorkspaceWithoutLink(t *testing.T) {
	previousYes := viper.Get("YES")
	viper.Set("YES", true)
	t.Cleanup(func() { viper.Set("YES", previousYes) })

	command := newProjectsCommand(defaultRuntimeContext())
	start, _, err := command.Find([]string{"start"})
	require.NoError(t, err)
	require.NoError(t, start.Flags().Set("workspace-id", "right-rowan-1539c71c"))

	err = start.PreRunE(start, nil)
	require.NoError(t, err)
}

func TestProjectsWorkspaceMutationRequiresWorkspaceWhenNoLink(t *testing.T) {
	previousYes := viper.Get("YES")
	viper.Set("YES", true)
	t.Cleanup(func() { viper.Set("YES", previousYes) })

	command := newProjectsCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	start, _, err := command.Find([]string{"start"})
	require.NoError(t, err)

	previousRegion := os.Getenv("VOLC_REGION")
	t.Setenv("VOLC_REGION", "cn-beijing")
	t.Cleanup(func() { _ = os.Setenv("VOLC_REGION", previousRegion) })
	err = start.PreRunE(start, nil)
	require.ErrorIs(t, err, errdefs.ErrMissingWorkspaceID)
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
	firstRename, _, err := first.Find([]string{"settings", "rename"})
	require.NoError(t, err)
	secondRename, _, err := second.Find([]string{"settings", "rename"})
	require.NoError(t, err)
	require.NotSame(t, firstRename, secondRename)

	require.NoError(t, firstRename.Flags().Set("name", "first"))
	require.Equal(t, "first", firstRename.Flag("name").Value.String())
	require.Equal(t, "", secondRename.Flag("name").Value.String())
}
