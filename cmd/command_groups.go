// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

type RuntimeContext = commandtree.RuntimeContext
type CommandFactory = commandtree.CommandFactory
type CommandGroup = commandtree.CommandGroup

func defaultRuntimeContext() RuntimeContext {
	return commandtree.DefaultRuntimeContext()
}

func installCommandGroup(root *cobra.Command, group CommandGroup, ctx RuntimeContext) {
	commandtree.InstallGroup(root, group, ctx)
}
