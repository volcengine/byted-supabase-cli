// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestInitFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newInitCommand(ctx)
	second := newInitCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "init", first.Name())
	require.NoError(t, first.Flags().Set("force", "true"))
	require.Equal(t, "true", first.Flag("force").Value.String())
	require.Equal(t, "false", second.Flag("force").Value.String())
}

func TestInitFactoryRequiresExperimentalForOrioleDB(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().Bool("experimental", false, "")
	command := newInitCommand(defaultRuntimeContext())
	root.AddCommand(command)

	require.NoError(t, command.Flags().Set("use-orioledb", "true"))
	err := command.PreRunE(command, nil)
	require.EqualError(t, err, "--use-orioledb requires --experimental")

	require.NoError(t, root.PersistentFlags().Set("experimental", "true"))
	require.NoError(t, command.PreRunE(command, nil))
}
