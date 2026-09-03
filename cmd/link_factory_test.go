// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestLinkFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{FS: afero.NewMemMapFs(), Output: discardWriter{}, Stderr: discardWriter{}}
	first := newLinkCommand(ctx)
	second := newLinkCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "link", first.Name())
	require.NoError(t, first.Flags().Set("workspace-id", "workspace-first"))
	require.NoError(t, first.Flags().Set("volc-project-name", "project-first"))
	require.Equal(t, "workspace-first", first.Flag("workspace-id").Value.String())
	require.Equal(t, "project-first", first.Flag("volc-project-name").Value.String())
	require.Equal(t, "", second.Flag("workspace-id").Value.String())
	require.Equal(t, "", second.Flag("volc-project-name").Value.String())
}
