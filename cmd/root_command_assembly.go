// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	generateFigSpec "github.com/withfig/autocomplete-tools/packages/cobra"
)

func rootCommandGroups() []CommandGroup {
	return []CommandGroup{
		managementAPICommandGroup(),
		localDevelopmentCommandGroup(),
		additionalCommandGroup(),
	}
}

func additionalCommandGroup() CommandGroup {
	return CommandGroup{
		Factories: []CommandFactory{
			newSkillsCommand,
			newUpdateCommand,
			newGenerateFigSpecCommand,
		},
	}
}

func newGenerateFigSpecCommand(ctx RuntimeContext) *cobra.Command {
	_ = ctx
	return generateFigSpec.NewCmdGenFigSpec()
}

func assembleRootCommand(root *cobra.Command, ctx RuntimeContext) {
	commandtree.Assemble(root, rootCommandGroups(), ctx)
	installSystemCommandFactories(root)
}
