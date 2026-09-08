// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

func TestInspectFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newInspectCommand(ctx)
	second := newInspectCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)

	firstDB, _, err := first.Find([]string{"db"})
	require.NoError(t, err)
	secondDB, _, err := second.Find([]string{"db"})
	require.NoError(t, err)
	require.NotSame(t, firstDB, secondDB)

	firstStats, _, err := first.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	secondStats, _, err := second.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	require.NotSame(t, firstStats, secondStats)
	require.NoError(t, firstStats.Flags().Set("branch-id", "branch-first"))
	require.Equal(t, "branch-first", firstStats.Flag("branch-id").Value.String())
	require.Equal(t, "", secondStats.Flag("branch-id").Value.String())
}

func TestInspectFactoryCommandTree(t *testing.T) {
	command := newInspectCommand(defaultRuntimeContext())
	require.Equal(t, "inspect", command.Name())
	require.Len(t, command.Commands(), 2)
	policy, err := ResolveAuthPolicy(command, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyNone, policy)
	require.Nil(t, command.Flags().Lookup("db-url"))
	require.Nil(t, command.PersistentFlags().Lookup("db-url"))

	db, _, err := command.Find([]string{"db"})
	require.NoError(t, err)
	require.NotEmpty(t, db.Commands())
	require.NotNil(t, command.CommandPath())
}

func TestInspectAuthPolicyUsesVolcengineForWorkspaceAndNoneForDBURL(t *testing.T) {
	command := newInspectCommand(defaultRuntimeContext())

	stats, _, err := command.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	policy, err := ResolveAuthPolicy(stats, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyVolcengine, policy)

	require.NoError(t, stats.Flags().Set("db-url", "postgres://user:password@localhost:5432/db"))
	policy, err = ResolveAuthPolicy(stats, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyNone, policy)

	report, _, err := command.Find([]string{"report"})
	require.NoError(t, err)
	policy, err = ResolveAuthPolicy(report, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyVolcengine, policy)

	require.NoError(t, report.Flags().Set("db-url", "postgres://user:password@localhost:5432/db"))
	policy, err = ResolveAuthPolicy(report, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyNone, policy)
}

func TestInspectLeafFlagsAreIsolated(t *testing.T) {
	command := newInspectCommand(defaultRuntimeContext())

	stats, _, err := command.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	report, _, err := command.Find([]string{"report"})
	require.NoError(t, err)

	for _, name := range []string{"db-url", "linked", "project-ref", "workspace-id", "branch-id"} {
		require.NotNil(t, stats.Flags().Lookup(name), "db-stats missing flag %q", name)
		require.NotNil(t, report.Flags().Lookup(name), "report missing flag %q", name)
	}
	require.NotNil(t, report.Flags().Lookup("output-dir"))
	require.Nil(t, stats.Flags().Lookup("output-dir"))

	require.NoError(t, stats.Flags().Set("branch-id", "branch-stats"))
	require.NoError(t, report.Flags().Set("branch-id", "branch-report"))
	require.Equal(t, "branch-stats", stats.Flag("branch-id").Value.String())
	require.Equal(t, "branch-report", report.Flag("branch-id").Value.String())
}

func TestInspectDBURLDoesNotRequireLinkedWorkspaceResolution(t *testing.T) {
	command := newInspectCommand(defaultRuntimeContext())
	leaf, _, err := command.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	require.NoError(t, leaf.Flags().Set("db-url", "postgres://user:password@localhost:5432/db"))

	require.NoError(t, preRunInspectFactory(leaf, afero.NewMemMapFs(), &inspectOptions{}))
}

func TestInspectDBURLUsesParsedDatabaseConfig(t *testing.T) {
	command := newInspectCommand(defaultRuntimeContext())
	leaf, _, err := command.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	require.NoError(t, leaf.Flags().Set("db-url", "postgres://user:password@localhost:5432/db"))

	expected := pgconn.Config{Host: "db-url-host", Port: 5432, Database: "db"}
	previous := flags.DbConfig
	flags.DbConfig = expected
	t.Cleanup(func() { flags.DbConfig = previous })

	var received pgconn.Config
	err = runInspectFactory(leaf, afero.NewMemMapFs(), &inspectOptions{}, func(_ context.Context, config pgconn.Config, _ afero.Fs, _ ...func(*pgx.ConnConfig)) error {
		received = config
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, expected, received)
}
