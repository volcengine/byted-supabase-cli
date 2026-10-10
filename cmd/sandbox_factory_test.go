// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestSandboxCommandTree(t *testing.T) {
	command := newSandboxCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "sandbox", command.Name())
	require.Equal(t, groupManagementAPI, command.GroupID)
	require.Equal(t, "Manage Supabase Sandboxes (Beta)", command.Short)

	names := make(map[string]bool)
	for _, child := range command.Commands() {
		names[child.Name()] = true
	}
	for _, name := range []string{"list", "create", "connect", "info", "kill", "logs", "metrics", "health", "exec", "timeout", "process", "quota"} {
		require.True(t, names[name], "missing Sandbox command %q", name)
	}
	require.False(t, names["templates"], "Sandbox must not own template commands")

	expectedGroups := map[string]string{
		"list":    sandboxGroupLifecycle,
		"create":  sandboxGroupLifecycle,
		"connect": sandboxGroupExecution,
		"info":    sandboxGroupLifecycle,
		"kill":    sandboxGroupLifecycle,
		"logs":    sandboxGroupObservability,
		"metrics": sandboxGroupObservability,
		"health":  sandboxGroupObservability,
		"exec":    sandboxGroupExecution,
		"timeout": sandboxGroupLifecycle,
		"files":   sandboxGroupFiles,
		"code":    sandboxGroupFiles,
		"port":    sandboxGroupObservability,
		"process": sandboxGroupExecution,
		"quota":   sandboxGroupQuota,
	}
	for name, groupID := range expectedGroups {
		child, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, groupID, child.GroupID, "unexpected group for Sandbox command %q", name)
	}
	processStart, _, err := command.Find([]string{"process", "start"})
	require.NoError(t, err)
	require.Empty(t, processStart.GroupID)

	list, _, err := command.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, list.Flag("workspace-id"))

	logs, _, err := command.Find([]string{"logs"})
	require.NoError(t, err)
	require.NotNil(t, logs.Flag("workspace-id"))
	require.NotNil(t, logs.Flag("branch-id"))
	require.NotNil(t, logs.Flag("sandbox-id"))

	health, _, err := command.Find([]string{"health"})
	require.NoError(t, err)
	require.NotNil(t, health.Flag("sandbox-id"))

	kill, _, err := command.Find([]string{"kill"})
	require.NoError(t, err)
	require.NotNil(t, kill.Flag("workspace-id"))
	require.NotNil(t, kill.Flag("branch-id"))
	require.NotNil(t, kill.Flag("sandbox-id"))

	process, _, err := command.Find([]string{"process", "start"})
	require.NoError(t, err)
	require.NotNil(t, process.Flag("sandbox-id"))
	require.NotNil(t, process.Flag("command"))

	quota, _, err := command.Find([]string{"quota"})
	require.NoError(t, err)
	quotaNames := make(map[string]bool)
	for _, child := range quota.Commands() {
		quotaNames[child.Name()] = true
	}
	for _, name := range []string{"get", "set", "users"} {
		require.True(t, quotaNames[name], "missing Sandbox quota command %q", name)
	}
}

func TestTemplatesCommandTree(t *testing.T) {
	command := newTemplatesCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	require.Equal(t, "templates", command.Name())
	list, _, err := command.Find([]string{"list"})
	require.NoError(t, err)
	require.Equal(t, "list", list.Name())
	require.NotNil(t, list.Flag("workspace-id"))
	require.NotNil(t, list.Flag("branch-id"))

	commandNames := make(map[string]bool)
	for _, child := range command.Commands() {
		commandNames[child.Name()] = true
	}
	require.False(t, commandNames["info"])
	require.False(t, commandNames["resolve"])
}

func TestSandboxCreateTemplateFlags(t *testing.T) {
	command := newSandboxCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	create, _, err := command.Find([]string{"create"})
	require.NoError(t, err)
	require.NotNil(t, create.Flag("template-id"))
	require.NotNil(t, create.Flag("template-name"))
}

func TestSandboxKillFilterFlags(t *testing.T) {
	command := newSandboxCommand(RuntimeContext{
		FS:     afero.NewMemMapFs(),
		Output: discardWriter{},
		Stderr: discardWriter{},
	})
	kill, _, err := command.Find([]string{"kill"})
	require.NoError(t, err)
	require.NotNil(t, kill.Flag("all"))
	require.NotNil(t, kill.Flag("state"))
	require.NotNil(t, kill.Flag("metadata"))
	require.NotNil(t, kill.Flag("sandbox-id"))
}
