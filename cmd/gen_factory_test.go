// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func TestGenFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newGenCommand(ctx)
	second := newGenCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, groupLocalDev, second.GroupID)

	firstTypes, _, err := first.Find([]string{"types"})
	require.NoError(t, err)
	secondTypes, _, err := second.Find([]string{"types"})
	require.NoError(t, err)
	require.NotSame(t, firstTypes, secondTypes)

	require.NoError(t, firstTypes.Flags().Set("branch-id", "branch-from-first-command"))
	require.Equal(t, "branch-from-first-command", firstTypes.Flag("branch-id").Value.String())
	require.Equal(t, "", secondTypes.Flag("branch-id").Value.String())
}

func TestGenFactoryCommandTree(t *testing.T) {
	command := newGenCommand(defaultRuntimeContext())
	require.Equal(t, "gen", command.Name())
	require.Len(t, command.Commands(), 1)
	require.Equal(t, "types", command.Commands()[0].Name())
}

func TestGenTypesUsesLinkedWorkspaceWithoutLinkedFlag(t *testing.T) {
	previousRegion := volcengine.RegionSetting()
	volcengine.SetRegionOverride("cn-beijing")
	t.Cleanup(func() {
		volcengine.SetRegionOverride(previousRegion)
	})

	fsys := afero.NewMemMapFs()
	require.NoError(t, utils.WriteFile(
		filepath.Join(utils.TempDir, volcengine.LinkedWorkspaceFile),
		[]byte("linked-workspace"),
		fsys,
	))

	command := newGenTypesCommand(RuntimeContext{FS: fsys})
	require.Nil(t, command.Flags().Lookup("linked"))
	require.NoError(t, command.PreRunE(command, nil))
}
