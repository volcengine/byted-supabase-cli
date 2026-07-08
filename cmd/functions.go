// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package cmd

import (
	"fmt"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/functions/delete"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	"github.com/volcengine/byted-supabase-cli/internal/functions/download"
	"github.com/volcengine/byted-supabase-cli/internal/functions/list"
	new_ "github.com/volcengine/byted-supabase-cli/internal/functions/new"
	"github.com/volcengine/byted-supabase-cli/internal/functions/serve"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/cast"
)

var (
	functionsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "functions",
		Short:   "Manage Supabase Edge functions",
	}

	functionsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all Functions in Supabase",
		Long:  "List all Functions in the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineFunctions(cmd, "Which project do you want to list Functions for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return list.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, functionsListBranchID)
		},
	}

	functionsListBranchID     string
	functionsDeleteBranchID   string
	functionsDownloadBranchID string
	functionsDeployBranchID   string
	functionsDeployRuntime    string
	functionsNewRuntime       string

	functionsDeleteCmd = &cobra.Command{
		Use:   "delete <Function name>",
		Short: "Delete a Function from Supabase",
		Long:  "Delete a Function from the linked Supabase project. This does NOT remove the Function locally.",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineFunctions(cmd, "Which project do you want to delete a Function from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return delete.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, functionsDeleteBranchID, args[0])
		},
	}

	functionsDownloadCmd = &cobra.Command{
		Use:   "download [Function name]",
		Short: "Download a Function from Supabase",
		Long:  "Download the source code for a Function from the linked Supabase project. If no function name is provided, downloads all functions.",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineFunctions(cmd, "Which project do you want to download Functions from?")
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
			return download.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, functionsDownloadBranchID, slug, afero.NewOsFs())
		},
	}

	noVerifyJWT   = new(bool)
	importMapPath string

	functionsDeployCmd = &cobra.Command{
		Use:   "deploy [Function name]",
		Short: "Deploy a Function to Supabase",
		Long:  "Deploy a Function to the linked Supabase project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineFunctions(cmd, "Which project do you want to deploy Functions to?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fallback to config if user did not set the flag.
			if !cmd.Flags().Changed("no-verify-jwt") {
				noVerifyJWT = nil
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return deploy.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, functionsDeployBranchID, args, functionsDeployRuntime, noVerifyJWT, importMapPath, afero.NewOsFs())
		},
	}

	functionsNewCmd = &cobra.Command{
		Use:   "new <Function name>",
		Short: "Create a new Function locally",
		Args:  cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.GroupID = groupLocalDev
			return cmd.Root().PersistentPreRunE(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return new_.Run(cmd.Context(), args[0], functionsNewRuntime, afero.NewOsFs())
		},
	}

	envFilePath string
	inspectBrk  bool
	inspectMode = utils.EnumFlag{
		Allowed: []string{
			string(serve.InspectModeRun),
			string(serve.InspectModeBrk),
			string(serve.InspectModeWait),
		},
	}
	runtimeOption serve.RuntimeOption

	functionsServeCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve all Functions locally",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.GroupID = groupLocalDev
			return cmd.Root().PersistentPreRunE(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fallback to config if user did not set the flag.
			if !cmd.Flags().Changed("no-verify-jwt") {
				noVerifyJWT = nil
			}

			if len(inspectMode.Value) > 0 {
				runtimeOption.InspectMode = cast.Ptr(serve.InspectMode(inspectMode.Value))
			} else if inspectBrk {
				runtimeOption.InspectMode = cast.Ptr(serve.InspectModeBrk)
			}
			if runtimeOption.InspectMode == nil && runtimeOption.InspectMain {
				return fmt.Errorf("--inspect-main must be used together with one of these flags: [inspect inspect-mode]")
			}

			return serve.Run(cmd.Context(), envFilePath, noVerifyJWT, importMapPath, runtimeOption, afero.NewOsFs())
		},
	}
)

func preRunVolcengineFunctions(cmd *cobra.Command, prompt string) error {
	linked := cmd.Flags().Changed("linked")
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
}

func init() {
	functionsListCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	functionsListCmd.Flags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	functionsListCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	functionsListCmd.Flags().StringVar(&functionsListBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	markFlagTelemetrySafe(functionsListCmd.Flags().Lookup("project-ref"))
	markFlagTelemetrySafe(functionsListCmd.Flags().Lookup("workspace-id"))
	functionsDeleteCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	functionsDeleteCmd.Flags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	functionsDeleteCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	functionsDeleteCmd.Flags().StringVar(&functionsDeleteBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	markFlagTelemetrySafe(functionsDeleteCmd.Flags().Lookup("project-ref"))
	markFlagTelemetrySafe(functionsDeleteCmd.Flags().Lookup("workspace-id"))
	deployFlags := functionsDeployCmd.Flags()
	deployFlags.Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	deployFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	deployFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	deployFlags.StringVar(&functionsDeployBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	deployFlags.StringVar(&functionsDeployRuntime, "runtime", deploy.RuntimeAuto, "Function runtime: auto, deno, native-node20/v1, python3.9, python3.10, python3.12.")
	deployFlags.BoolVar(noVerifyJWT, "no-verify-jwt", false, "Disable JWT verification for the Function.")
	markFlagTelemetrySafe(deployFlags.Lookup("project-ref"))
	markFlagTelemetrySafe(deployFlags.Lookup("workspace-id"))
	deployFlags.StringVar(&importMapPath, "import-map", "", "Path to Deno import map file. Only applies to Deno Functions.")
	functionsServeCmd.Flags().BoolVar(noVerifyJWT, "no-verify-jwt", false, "Disable JWT verification for the Function.")
	functionsServeCmd.Flags().StringVar(&envFilePath, "env-file", "", "Path to an env file to be populated to the Function environment.")
	functionsServeCmd.Flags().StringVar(&importMapPath, "import-map", "", "Path to Deno import map file. Only applies to Deno Functions.")
	functionsServeCmd.Flags().BoolVar(&inspectBrk, "inspect", false, "Alias of --inspect-mode brk.")
	functionsServeCmd.Flags().Var(&inspectMode, "inspect-mode", "Activate inspector capability for debugging.")
	functionsServeCmd.Flags().BoolVar(&runtimeOption.InspectMain, "inspect-main", false, "Allow inspecting the main worker.")
	functionsServeCmd.MarkFlagsMutuallyExclusive("inspect", "inspect-mode")
	functionsServeCmd.Flags().Bool("all", true, "Serve all Functions.")
	cobra.CheckErr(functionsServeCmd.Flags().MarkHidden("all"))
	functionsNewCmd.Flags().StringVar(&functionsNewRuntime, "runtime", deploy.RuntimeDeno, "Function runtime: deno, native-node20/v1, python3.9, python3.10, python3.12.")
	downloadFlags := functionsDownloadCmd.Flags()
	downloadFlags.Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	downloadFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	downloadFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	downloadFlags.StringVar(&functionsDownloadBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	markFlagTelemetrySafe(downloadFlags.Lookup("project-ref"))
	markFlagTelemetrySafe(downloadFlags.Lookup("workspace-id"))
	functionsCmd.AddCommand(functionsListCmd)
	functionsCmd.AddCommand(functionsDeleteCmd)
	functionsCmd.AddCommand(functionsDeployCmd)
	functionsCmd.AddCommand(functionsNewCmd)
	// Volcengine does not support local Edge Runtime/local stack in this phase.
	// Keep the implementation for future local development support, but do not register it.
	// functionsCmd.AddCommand(functionsServeCmd)
	functionsCmd.AddCommand(functionsDownloadCmd)
	rootCmd.AddCommand(functionsCmd)
}
