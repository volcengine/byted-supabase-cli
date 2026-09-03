// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestUnlinkFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newUnlinkCommand(ctx)
	second := newUnlinkCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "unlink", first.Name())
	require.Empty(t, first.Commands())
}
