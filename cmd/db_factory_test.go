// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

func TestDBFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newDBCommand(ctx)
	second := newDBCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "db", first.Name())

	firstQuery, _, err := first.Find([]string{"query"})
	require.NoError(t, err)
	secondQuery, _, err := second.Find([]string{"query"})
	require.NoError(t, err)
	require.NotSame(t, firstQuery, secondQuery)
	require.NoError(t, firstQuery.Flags().Set("workspace-id", "workspace-first"))
	require.Equal(t, "workspace-first", firstQuery.Flag("workspace-id").Value.String())
	require.Equal(t, "", secondQuery.Flag("workspace-id").Value.String())
}

func TestDBFactoryCommandTree(t *testing.T) {
	command := newDBCommand(defaultRuntimeContext())
	for _, name := range []string{"connection-string", "query", "advisors", "api-config", "dump", "pull", "lint"} {
		found, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, name, found.Name())
	}
	apiConfig, _, err := command.Find([]string{"api-config"})
	require.NoError(t, err)
	require.Len(t, apiConfig.Commands(), 2)

	lint, _, err := command.Find([]string{"lint"})
	require.NoError(t, err)
	require.Nil(t, lint.Flag("linked"))
	require.Nil(t, lint.Flag("local"))
	require.Nil(t, lint.Flag("workspace-id"))
	require.Nil(t, lint.Flag("project-ref"))
	require.NotNil(t, lint.Flag("db-url"))
}

func TestDBWorkspaceFlagsAllowExplicitWorkspaceWithoutLinkedFlag(t *testing.T) {
	command := newDBCommand(defaultRuntimeContext())
	query, _, err := command.Find([]string{"query"})
	require.NoError(t, err)

	require.NoError(t, query.Flags().Set("workspace-id", "workspace-first"))
	require.False(t, query.Flags().Changed("linked"))
	require.NotNil(t, query.Flag("linked"))
}

func TestDBURLDoesNotConflictWithUnchangedLinkedDefault(t *testing.T) {
	command := newDBCommand(defaultRuntimeContext())
	query, _, err := command.Find([]string{"query"})
	require.NoError(t, err)

	require.NoError(t, query.Flags().Set("db-url", "postgresql://user:pass@localhost:5432/db"))
	options := &dbWorkspaceOptions{DBURL: query.Flag("db-url").Value.String(), Linked: true}
	require.NoError(t, prepareDBWorkspace(query, afero.NewMemMapFs(), options, ""))
}

func TestDBExplicitWorkspaceTakesPrecedenceOverGlobalProjectRef(t *testing.T) {
	command := newDBCommand(defaultRuntimeContext())
	dump, _, err := command.Find([]string{"dump"})
	require.NoError(t, err)
	require.NoError(t, dump.Flags().Set("workspace-id", "jolly-apple-94b8b75n"))
	require.NoError(t, dump.Flags().Set("branch-id", "br-dear-puma-c36cb238"))
	require.NoError(t, dump.Flags().Set("linked", "false"))

	previous := flags.ProjectRef
	flags.ProjectRef = "svc-sunny-flake-cf8489c0"
	t.Cleanup(func() { flags.ProjectRef = previous })

	options := &dbWorkspaceOptions{
		WorkspaceID: dump.Flag("workspace-id").Value.String(),
		BranchID:    dump.Flag("branch-id").Value.String(),
		Linked:      false,
	}
	require.NoError(t, prepareDBWorkspace(dump, afero.NewMemMapFs(), options, ""))
	require.Equal(t, "jolly-apple-94b8b75n", options.WorkspaceID)
	require.Equal(t, "br-dear-puma-c36cb238", options.BranchID)
	require.Equal(t, "svc-sunny-flake-cf8489c0", flags.ProjectRef)
}
