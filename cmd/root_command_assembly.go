// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	solocommand "github.com/volcengine/byted-supabase-cli/cmd/solo"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/link"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	generateFigSpec "github.com/withfig/autocomplete-tools/packages/cobra"
)

func rootCommandGroups() []CommandGroup {
	return []CommandGroup{
		managementAPICommandGroup(),
		localDevelopmentCommandGroup(selectLinkCommandFactory()),
		additionalCommandGroup(),
	}
}

func selectLinkCommandFactory() CommandFactory {
	if linkIntentFromArgs() == linkIntentDirectDataPlane {
		return newDirectDataPlaneLinkCommand
	}
	return newManagementLinkCommand
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

func buildRootCommand() (*cobra.Command, error) {
	ctx := commandtree.DefaultRuntimeContext()
	profile, err := ResolveRuntimeMode(ctx)
	if err != nil {
		return buildRuntimeErrorRoot(ctx, err), nil
	}

	root := newRootCommand()
	configureRootCommand(root, profile)
	assembleRootCommand(root, profile, ctx)
	return root, nil
}

func buildRuntimeErrorRoot(ctx RuntimeContext, err error) *cobra.Command {
	root := newRootCommand()
	configureRootCommand(root, RuntimeProfile{Mode: RuntimeModeManager})
	root.Long = fmt.Sprintf(
		"The current runtime mode could not be resolved: %v\n\n"+
			"Fix the link or credentials, then retry.",
		err,
	)
	root.AddCommand(newDirectDataPlaneLinkCommand(ctx), newUnlinkCommand(ctx))
	return root
}

func assembleRootCommand(root *cobra.Command, profile RuntimeProfile, ctx RuntimeContext) {
	if profile.Mode == RuntimeModeSolo {
		root.Long = "Linked directly to a Supabase branch with the URL and keys you supplied.\n" +
			"Management commands are unavailable here. To go back, run:\n" +
			"  byted-supabase-cli unlink"
		AssembleSoloTree(root, profile, ctx)
		return
	}
	root.Long = "Manage Volcengine Supabase workspaces and branches, and develop locally."
	if soloLinkExistsButInvalid() {
		root.Long += fmt.Sprintf("\n\nThe branch link is corrupted or incomplete: %v\n"+
			"Run `byted-supabase-cli link --supabase-url <url> --service-role-key <key>` to relink.",
			soloLinkValidationError())
	}
	AssembleManagerTree(root, profile, ctx)
}

func soloLinkExistsButInvalid() bool {
	if _, configured, err := soloconfig.LoadEnvironmentProfile(); configured {
		if err != nil {
			return true
		}
		if link.IsSoloAtDir(runtimeWorkdirFromArgs()) {
			return soloconfig.NewResolver(runtimeWorkdirFromArgs()).Validate() != nil
		}
		return false
	}
	projectDir := runtimeWorkdirFromArgs()
	if _, err := os.Stat(link.StatePathForDir(projectDir)); err == nil {
		return soloconfig.NewResolver(projectDir).Validate() != nil
	}
	return false
}

func soloLinkValidationError() error {
	return soloconfig.NewResolver(runtimeWorkdirFromArgs()).Validate()
}

func runtimeWorkdirFromArgs() string {
	args := os.Args[1:]
	for index, arg := range args {
		if arg == "--workdir" && index+1 < len(args) {
			return filepath.Clean(args[index+1])
		}
		if strings.HasPrefix(arg, "--workdir=") {
			return filepath.Clean(strings.TrimPrefix(arg, "--workdir="))
		}
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return workingDir
}

func installSoloCommandTree(root *cobra.Command, ctx RuntimeContext) {
	soloRoot := solocommand.NewCommandTree(ctx)
	for _, command := range soloRoot.Commands() {
		root.AddCommand(command)
	}
	root.AddCommand(newDirectDataPlaneLinkCommand(ctx), newUnlinkCommand(ctx))
}

func AssembleSoloTree(root *cobra.Command, profile RuntimeProfile, ctx RuntimeContext) {
	ctx.Mode = string(profile.Mode)
	installSoloCommandTree(root, ctx)
}

func AssembleManagerTree(root *cobra.Command, profile RuntimeProfile, ctx RuntimeContext) {
	ctx.Mode = string(profile.Mode)
	commandtree.Assemble(root, rootCommandGroups(), ctx)
	installSystemCommandFactories(root)
}
