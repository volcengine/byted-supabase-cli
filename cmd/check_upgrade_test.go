// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestUpdateCommandMounted(t *testing.T) {
	// The plain Volcengine build ships the update command, so the upgrade check
	// (and its nag) run.
	assert.True(t, updateCommandMounted())
}

func TestUpdateCommandMountedWhenPruned(t *testing.T) {
	// A distribution whose release channel is not the Volcengine npm registry
	// prunes the update command via the distribution seam. Once it is gone the
	// helper must report it absent, so Execute skips the check — no network
	// fetch, no cache write, and no nag pointing at a command that no longer
	// exists.
	rootCmd.RemoveCommand(updateCmd)
	t.Cleanup(func() { rootCmd.AddCommand(updateCmd) })

	assert.False(t, updateCommandMounted())
}

func TestUpdateCommandMountedWithDistributionUpdate(t *testing.T) {
	// A distribution may prune the built-in update command and mount its own
	// command that is also named "update" but targets a different release
	// channel. The helper matches by identity, so the distribution's namesake
	// must not re-enable the Volcengine npm upgrade check.
	distUpdate := &cobra.Command{Use: "update"}
	rootCmd.RemoveCommand(updateCmd)
	rootCmd.AddCommand(distUpdate)
	t.Cleanup(func() {
		rootCmd.RemoveCommand(distUpdate)
		rootCmd.AddCommand(updateCmd)
	})

	assert.False(t, updateCommandMounted())
}
