// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestLoginFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		Input:  strings.NewReader(""),
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newLoginCommand(ctx)
	second := newLoginCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, "login", first.Name())
	require.NoError(t, first.Flags().Set("profile", "first"))
	require.Equal(t, "first", first.Flag("profile").Value.String())
	require.Equal(t, "default", second.Flag("profile").Value.String())
}
