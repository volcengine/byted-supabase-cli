// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayCommandGroup(t *testing.T) {
	group := aiGatewayCommandGroup()
	require.Nil(t, group.Group)
	require.Len(t, group.Factories, 1)

	command := group.Factories[0](RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "ai-gateway", command.Name())
	require.Equal(t, groupManagementAPI, command.GroupID)
	require.NotNil(t, command.Commands())

	names := make(map[string]bool)
	for _, child := range command.Commands() {
		names[child.Name()] = true
	}
	for _, name := range []string{"status", "config", "api-keys", "models", "quota", "usage", "chat", "test", "init"} {
		require.True(t, names[name], "missing AI Gateway command %q", name)
	}

	quota, _, err := command.Find([]string{"quota"})
	require.NoError(t, err)
	quotaNames := make(map[string]bool)
	for _, child := range quota.Commands() {
		quotaNames[child.Name()] = true
	}
	for _, name := range []string{"get", "set-default", "users"} {
		require.True(t, quotaNames[name], "missing AI Gateway quota command %q", name)
	}

	users, _, err := command.Find([]string{"quota", "users"})
	require.NoError(t, err)
	userNames := make(map[string]bool)
	for _, child := range users.Commands() {
		userNames[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "set"} {
		require.True(t, userNames[name], "missing AI Gateway quota users command %q", name)
	}
}

func TestAIGatewayFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newAIGatewayCommand(ctx)
	second := newAIGatewayCommand(ctx)

	require.NotSame(t, first, second)
	firstCompletions, _, err := first.Find([]string{"chat", "completions"})
	require.NoError(t, err)
	secondCompletions, _, err := second.Find([]string{"chat", "completions"})
	require.NoError(t, err)
	require.NotSame(t, firstCompletions, secondCompletions)

	require.NoError(t, firstCompletions.Flags().Set("model", "model-from-first-command"))
	require.Equal(t, "model-from-first-command", firstCompletions.Flag("model").Value.String())
	require.Equal(t, "bytedance/doubao-seed-2.0-mini", secondCompletions.Flag("model").Value.String())
}

type discardWriter struct{}

func (discardWriter) Write(data []byte) (int, error) {
	return len(data), nil
}
