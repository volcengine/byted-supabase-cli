// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
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
	for _, group := range rootCommandGroups() {
		installCommandGroup(root, group, ctx)
	}
	installSystemCommandFactories(root)
}

func init() {
	assembleRootCommand(rootCmd, defaultRuntimeContext())
}
