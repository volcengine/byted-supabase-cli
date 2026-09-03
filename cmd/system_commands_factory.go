// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import "github.com/spf13/cobra"

func installSystemCommandFactories(root *cobra.Command) {
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	for _, command := range root.Commands() {
		switch command.Name() {
		case "help":
			command.Short = "Help About Any Command"
		case "completion":
			command.Short = "Generate the Autocompletion Script for the Specified Shell"
		}
	}
}
