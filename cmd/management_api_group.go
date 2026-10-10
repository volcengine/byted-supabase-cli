// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

func managementAPICommandGroup() CommandGroup {
	return CommandGroup{
		Group: nil,
		Factories: []CommandFactory{
			newAuthCommand,
			newAIGatewayCommand,
			newSandboxCommand,
			newTemplatesCommand,
			newBranchesCommandFactory,
			newComputesCommand,
			newEndpointsCommand,
			newFunctionsCommand,
			newNetworkRestrictionsCommand,
			newPagesCommand,
			newProjectsCommand,
			newSecretsCommand,
			newSeedCommand,
			newStorageCommand,
		},
	}
}
