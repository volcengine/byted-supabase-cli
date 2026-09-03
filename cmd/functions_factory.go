// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/functions/delete"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	"github.com/volcengine/byted-supabase-cli/internal/functions/download"
	"github.com/volcengine/byted-supabase-cli/internal/functions/list"
	new_ "github.com/volcengine/byted-supabase-cli/internal/functions/new"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type functionsOptions struct {
	WorkspaceID string
	Linked      bool
	BranchID    string
}

func functionsCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newFunctionsCommand}}
}

func newFunctionsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "functions",
		Short:   "Manage Supabase Edge Functions",
	}
	cmd.AddCommand(
		newFunctionsListCommand(ctx),
		newFunctionsDeleteCommand(ctx),
		newFunctionsDeployCommand(ctx),
		newFunctionsNewCommand(ctx),
		newFunctionsDownloadCommand(ctx),
	)
	return cmd
}

func newFunctionsWorkspaceFlags(cmd *cobra.Command, options *functionsOptions) {
	cmd.Flags().BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("project-ref"))
	markFlagTelemetrySafe(cmd.Flags().Lookup("workspace-id"))
}

func prepareFunctionsWorkspace(cmd *cobra.Command, options *functionsOptions, prompt string) error {
	if options.Linked && (cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")) {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	previous := flags.ProjectRef
	if options.WorkspaceID != "" {
		flags.ProjectRef = options.WorkspaceID
	}
	err := loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
	if err == nil {
		options.WorkspaceID = flags.ProjectRef
	}
	flags.ProjectRef = previous
	return err
}

func newFunctionsListCommand(ctx RuntimeContext) *cobra.Command {
	var options functionsOptions
	cmd := &cobra.Command{
		Use: "list", Short: "List all Functions in Supabase", Long: "List all Functions in the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareFunctionsWorkspace(cmd, &options, "Which project do you want to list Functions for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return list.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), options.WorkspaceID, options.BranchID)
		},
	}
	newFunctionsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	_ = ctx
	return cmd
}

func newFunctionsDeleteCommand(ctx RuntimeContext) *cobra.Command {
	var options functionsOptions
	cmd := &cobra.Command{
		Use: "delete <Function name>", Short: "Delete a Function from Supabase",
		Long: "Delete a Function from the linked Supabase project. This does NOT remove the Function locally.",
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareFunctionsWorkspace(cmd, &options, "Which project do you want to delete a Function from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return delete.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), options.WorkspaceID, options.BranchID, args[0])
		},
	}
	newFunctionsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	_ = ctx
	return cmd
}

func newFunctionsDeployCommand(ctx RuntimeContext) *cobra.Command {
	var options functionsOptions
	var runtime, importMap string
	var noVerifyJWT bool
	cmd := &cobra.Command{
		Use: "deploy [Function name]", Short: "Deploy a Function to Supabase",
		Long: "Deploy a Function to the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareFunctionsWorkspace(cmd, &options, "Which project do you want to deploy Functions to?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			var jwt *bool
			if cmd.Flags().Changed("no-verify-jwt") {
				jwt = &noVerifyJWT
			}
			return deploy.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), options.WorkspaceID, options.BranchID, args, runtime, jwt, importMap, afero.NewOsFs())
		},
	}
	newFunctionsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&runtime, "runtime", deploy.RuntimeAuto, "Function runtime: auto, deno, native-node20/v1, python3.9, python3.10, python3.12.")
	cmd.Flags().BoolVar(&noVerifyJWT, "no-verify-jwt", false, "Disable JWT verification for the Function.")
	cmd.Flags().StringVar(&importMap, "import-map", "", "Path to Deno import map file. Only applies to Deno Functions.")
	_ = ctx
	return cmd
}

func newFunctionsNewCommand(ctx RuntimeContext) *cobra.Command {
	var runtime string
	cmd := &cobra.Command{
		Use: "new <Function name>", Short: "Create a new Function locally", Args: cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.GroupID = groupLocalDev
			return cmd.Root().PersistentPreRunE(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return new_.Run(cmd.Context(), args[0], runtime, afero.NewOsFs())
		},
	}
	cmd.Flags().StringVar(&runtime, "runtime", deploy.RuntimeDeno, "Function runtime: deno, native-node20/v1, python3.9, python3.10, python3.12.")
	_ = ctx
	return cmd
}

func newFunctionsDownloadCommand(ctx RuntimeContext) *cobra.Command {
	var options functionsOptions
	cmd := &cobra.Command{
		Use: "download [Function name]", Short: "Download a Function from Supabase",
		Long: "Download the source code for a Function from the linked Supabase project. If no function name is provided, downloads all functions.",
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareFunctionsWorkspace(cmd, &options, "Which project do you want to download Functions from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := ""
			if len(args) > 0 {
				slug = args[0]
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return download.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), options.WorkspaceID, options.BranchID, slug, afero.NewOsFs())
		},
	}
	newFunctionsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	_ = ctx
	return cmd
}
