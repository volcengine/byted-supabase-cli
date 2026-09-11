// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

func localDevelopmentCommandGroup(linkFactory CommandFactory) CommandGroup {
	return CommandGroup{
		Group: nil,
		Factories: []CommandFactory{
			newConfigureCommand,
			newGenCommand,
			newInitCommand,
			newLoginCommand,
			newLogoutCommand,
			linkFactory,
			newMCPCommand,
			newInspectCommand,
			newDBCommand,
			newUnlinkCommand,
		},
	}
}
