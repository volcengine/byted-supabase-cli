// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestConfigureFactoryCreatesIndependentCommands(t *testing.T) {
	ctx := RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}
	first := newConfigureCommand(ctx)
	second := newConfigureCommand(ctx)

	require.NotSame(t, first, second)
	require.Equal(t, groupLocalDev, first.GroupID)
	require.Equal(t, groupLocalDev, second.GroupID)

	firstSet, _, err := first.Find([]string{"set"})
	require.NoError(t, err)
	secondSet, _, err := second.Find([]string{"set"})
	require.NoError(t, err)
	require.NotSame(t, firstSet, secondSet)

	require.NoError(t, first.PersistentFlags().Set("profile", "first"))
	require.Equal(t, "first", first.PersistentFlags().Lookup("profile").Value.String())
	require.Equal(t, "", second.PersistentFlags().Lookup("profile").Value.String())
}

func TestConfigureFactoryCommandTree(t *testing.T) {
	command := newConfigureCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	names := make(map[string]bool)
	for _, child := range command.Commands() {
		names[child.Name()] = true
	}
	for _, name := range []string{"set", "get", "list", "profile", "region", "agent-plan", "delete"} {
		require.True(t, names[name], "missing configure command %q", name)
	}
}

func TestConfigureCommandWithoutSubcommandShowsHelp(t *testing.T) {
	command := newConfigureCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs(nil)

	_, err := command.ExecuteC()
	require.NoError(t, err)
	require.Contains(t, output.String(), "Available Commands:")
}

func TestConfigureAgentPlanWithoutFlagsShowsHelp(t *testing.T) {
	command := newConfigureAgentPlanCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	}, &configureParentOptions{})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs(nil)

	_, err := command.ExecuteC()
	require.NoError(t, err)
	require.Contains(t, output.String(), "Usage:")
	require.Contains(t, output.String(), "agent-plan")
}

func TestConfigureLeafOptionsAreIsolated(t *testing.T) {
	parent := &configureParentOptions{}
	setCommand := newConfigureSetCommand(RuntimeContext{}, parent)
	agentPlanCommand := newConfigureAgentPlanCommand(RuntimeContext{}, parent)

	require.NotNil(t, setCommand.Flags().Lookup("access-key"))
	require.NotNil(t, setCommand.Flags().Lookup("region"))
	require.Nil(t, agentPlanCommand.Flags().Lookup("access-key"))
	require.Nil(t, agentPlanCommand.Flags().Lookup("region"))
	require.NotNil(t, agentPlanCommand.Flags().Lookup("is-agent-plan"))
	require.Nil(t, setCommand.PersistentFlags().Lookup("profile"))
}
