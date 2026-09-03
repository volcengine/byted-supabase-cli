// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestLogoutFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newLogoutCommand(ctx)
	second := newLogoutCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "logout", first.Name())
	require.NoError(t, first.Flags().Set("profile", "first"))
	require.NoError(t, first.Flags().Set("all", "true"))
	require.Equal(t, "first", first.Flag("profile").Value.String())
	require.Equal(t, "true", first.Flag("all").Value.String())
	require.Equal(t, "default", second.Flag("profile").Value.String())
	require.Equal(t, "false", second.Flag("all").Value.String())
}
