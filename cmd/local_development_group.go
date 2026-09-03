// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

func localDevelopmentCommandGroup() CommandGroup {
	return CommandGroup{
		Group: nil,
		Factories: []CommandFactory{
			newConfigureCommand,
			newGenCommand,
			newInitCommand,
			newLoginCommand,
			newLogoutCommand,
			newLinkCommand,
			newMCPCommand,
			newInspectCommand,
			newDBCommand,
			newUnlinkCommand,
		},
	}
}
