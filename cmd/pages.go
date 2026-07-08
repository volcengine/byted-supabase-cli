// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/pages"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	pagesBranchID       string
	pagesName           string
	pagesLimit          int
	pagesOffset         int
	pagesShowValues     bool
	pagesCustomPrefix   string
	pagesFramePrefix    string
	pagesResourceID     string
	pagesProvider       string
	pagesScope          string
	pagesFramework      string
	pagesRootDir        string
	pagesOutputDir      string
	pagesBuildCmd       string
	pagesInstallCmd     string
	pagesNodejsVer      string
	pagesNoDeploy       bool
	pagesFastFilePath   string
	pagesFunctionsInit  string
	pagesMigrationsInit string

	pagesCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "pages",
		Short:   "Manage IGA Pages integrations for Volcengine Supabase",
	}

	pagesFastCmd = &cobra.Command{
		Use:   "fast",
		Short: "Run opinionated Pages workflows",
	}

	pagesFastCreateCmd = &cobra.Command{
		Use:   "create <project-name>",
		Short: "Create a new Pages project and Supabase workspace from a frontend directory or archive",
		Long: `Create a new IGA Pages project and a new Volcengine Supabase workspace from a frontend directory or deployment archive.

This command uploads a Pages deployment archive, creates a Pages project, creates a Supabase workspace with the same project name, waits for the workspace/default branch to become ready, optionally applies SQL migrations, optionally deploys Edge Functions, binds the Pages project to Supabase, and creates the final Pages deployment.

Demo app:
  ` + pages.DemoAppArchiveURL + `

To deploy a structured app, organize it as:

  frontend/   Frontend files to package and upload to Pages.
  backend/    Backend files. Edge Functions should be under backend/functions/<slug>/.
  migration/  SQL migration files.

Then run:

  pages fast create <project_name> \
    --file-path /path/to/frontend \
    --framework-prefix NEXT_PUBLIC_ \
    --functions-init /path/to/backend \
    --migrations-init /path/to/migration

--functions-init and --migrations-init are optional. If --file-path is a directory, it is packaged into a temporary zip before upload. If --file-path is already a zip file, it is uploaded directly.

Project name must contain only lowercase letters, numbers, and hyphens.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			return pages.FastCreate(cmd.Context(), pages.FastCreateParams{
				ProjectName:     args[0],
				FilePath:        pagesFastFilePath,
				CustomPrefix:    pagesCustomPrefix,
				FrameworkPrefix: pagesFramePrefix,
				FunctionsInit:   pagesFunctionsInit,
				MigrationsInit:  pagesMigrationsInit,
			})
		},
	}

	pagesListCmd = &cobra.Command{
		Use:   "list",
		Short: "List IGA Pages projects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(pagesLimit, pagesOffset, false); err != nil {
				return err
			}
			if pagesOffset%pagesLimit != 0 {
				return errors.New("--offset must be a multiple of --limit for Pages pagination")
			}
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			return pages.List(cmd.Context(), pages.ListParams{
				Name:        pagesName,
				Limit:       pagesLimit,
				Offset:      pagesOffset,
				WorkspaceID: flags.ProjectRef,
				BranchID:    pagesBranchID,
			})
		},
	}

	pagesEnvVarsCmd = &cobra.Command{
		Use:   "env-vars",
		Short: "List Supabase environment variables available for IGA Pages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolvePagesWorkspace(cmd, "Which project do you want to list Pages env vars for?"); err != nil {
				return err
			}
			return pages.EnvVars(cmd.Context(), pages.ScopedParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    pagesBranchID,
				ShowValues:  pagesShowValues,
			})
		},
	}

	pagesBindingCmd = &cobra.Command{
		Use:   "binding",
		Short: "Show the IGA Pages binding for a Supabase branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolvePagesWorkspace(cmd, "Which project do you want to show Pages binding for?"); err != nil {
				return err
			}
			return pages.Binding(cmd.Context(), pages.ScopedParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    pagesBranchID,
			})
		},
	}

	pagesBindCmd = &cobra.Command{
		Use:   "bind <pages-project-id>",
		Short: "Bind an IGA Pages project to a Supabase branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolvePagesWorkspace(cmd, "Which project do you want to bind Pages for?"); err != nil {
				return err
			}
			return pages.Bind(cmd.Context(), pages.BindParams{
				WorkspaceID:     flags.ProjectRef,
				BranchID:        pagesBranchID,
				PagesProjectID:  args[0],
				CustomPrefix:    pagesCustomPrefix,
				FrameworkPrefix: pagesFramePrefix,
			})
		},
	}

	pagesCreateCmd = &cobra.Command{
		Use:   "create <project-name>",
		Short: "Create an IGA Pages project",
		Long:  "Create an IGA Pages project. Project name must contain only lowercase letters, numbers, and hyphens.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pages.Create(cmd.Context(), pages.CreateParams{
				Name:                    args[0],
				Provider:                pagesProvider,
				Scope:                   pagesScope,
				ProjectDeployResourceID: pagesResourceID,
				Framework:               pagesFramework,
				RootDir:                 pagesRootDir,
				OutputDir:               pagesOutputDir,
				BuildCmd:                pagesBuildCmd,
				InstallCmd:              pagesInstallCmd,
				NodejsVersion:           pagesNodejsVer,
				NoDeploy:                pagesNoDeploy,
			})
		},
	}

	pagesUploadCmd = &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload an IGA Pages deployment resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pages.Upload(cmd.Context(), pages.UploadParams{
				FilePath: args[0],
			})
		},
	}

	pagesDeployCmd = &cobra.Command{
		Use:   "deploy <pages-project-id>",
		Short: "Create an IGA Pages deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pages.Deploy(cmd.Context(), pages.DeployParams{
				PagesProjectID:          args[0],
				ProjectDeployResourceID: pagesResourceID,
			})
		},
	}

	pagesDeployListCmd = &cobra.Command{
		Use:   "list <pages-project-id>",
		Short: "List IGA Pages deployments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(pagesLimit, pagesOffset, false); err != nil {
				return err
			}
			if pagesOffset%pagesLimit != 0 {
				return errors.New("--offset must be a multiple of --limit for Pages deploy pagination")
			}
			return pages.DeployList(cmd.Context(), pages.DeployListParams{
				PagesProjectID: args[0],
				Limit:          pagesLimit,
				Offset:         pagesOffset,
			})
		},
	}

	pagesSyncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Sync Supabase environment variables to the bound IGA Pages project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolvePagesWorkspace(cmd, "Which project do you want to sync Pages env vars for?"); err != nil {
				return err
			}
			return pages.Sync(cmd.Context(), pages.SyncParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    pagesBranchID,
			})
		},
	}

	pagesUnbindCmd = &cobra.Command{
		Use:   "unbind",
		Short: "Unbind IGA Pages from a Supabase branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolvePagesWorkspace(cmd, "Which project do you want to unbind Pages from?"); err != nil {
				return err
			}
			return pages.Unbind(cmd.Context(), pages.ScopedParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    pagesBranchID,
			})
		},
	}
)

func init() {
	listFlags := pagesListCmd.Flags()
	listFlags.StringVar(&pagesName, "name", "", "Pages project name filter.")
	listFlags.IntVar(&pagesLimit, "limit", volcengineDefaultListLimit, "Maximum number of Pages projects to return (1-100).")
	listFlags.IntVar(&pagesOffset, "offset", 0, "Number of Pages projects to skip. Must be a multiple of --limit.")
	addPagesWorkspaceFlags(pagesListCmd, "Optional Supabase workspace filter. Does not default to linked project.")
	listFlags.StringVar(&pagesBranchID, "branch-id", "", "Optional Supabase branch filter.")

	addPagesWorkspaceFlags(pagesEnvVarsCmd, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	pagesEnvVarsCmd.Flags().StringVar(&pagesBranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	pagesEnvVarsCmd.Flags().BoolVar(&pagesShowValues, "show-values", false, "Show sensitive environment variable values. By default values are masked.")

	addPagesWorkspaceFlags(pagesBindingCmd, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	pagesBindingCmd.Flags().StringVar(&pagesBranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")

	addPagesWorkspaceFlags(pagesBindCmd, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	pagesBindCmd.Flags().StringVar(&pagesBranchID, "branch-id", "", "Branch ID to bind. Defaults to the workspace default branch.")
	pagesBindCmd.Flags().StringVar(&pagesCustomPrefix, "custom-prefix", "", "Environment variable prefix for binding a Pages project already used by another Supabase branch.")
	pagesBindCmd.Flags().StringVar(&pagesFramePrefix, "framework-prefix", "", "Framework client prefix for browser-exposed Supabase env vars.")

	addPagesWorkspaceFlags(pagesSyncCmd, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	pagesSyncCmd.Flags().StringVar(&pagesBranchID, "branch-id", "", "Branch ID to sync. Defaults to the workspace default branch.")

	addPagesWorkspaceFlags(pagesUnbindCmd, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	pagesUnbindCmd.Flags().StringVar(&pagesBranchID, "branch-id", "", "Branch ID to unbind. Defaults to the workspace default branch.")

	pagesCreateFlags := pagesCreateCmd.Flags()
	pagesCreateFlags.StringVar(&pagesProvider, "provider", "upload_v2", "Pages project provider. Currently only upload_v2 is supported.")
	pagesCreateFlags.StringVar(&pagesScope, "scope", "domestic", "Pages project scope: domestic, overseas, or global.")
	pagesCreateFlags.StringVar(&pagesResourceID, "resource-id", "", "ProjectDeployResourceID returned by `pages upload`.")
	pagesCreateFlags.StringVar(&pagesFramework, "framework", "", "Pages project framework.")
	pagesCreateFlags.StringVar(&pagesRootDir, "root-dir", "", "Pages project root directory.")
	pagesCreateFlags.StringVar(&pagesOutputDir, "output-dir", "", "Pages project output directory.")
	pagesCreateFlags.StringVar(&pagesBuildCmd, "build-cmd", "", "Pages project build command.")
	pagesCreateFlags.StringVar(&pagesInstallCmd, "install-cmd", "", "Pages project install command.")
	pagesCreateFlags.StringVar(&pagesNodejsVer, "nodejs-version", "", "Pages project Node.js version.")
	pagesCreateFlags.BoolVar(&pagesNoDeploy, "no-deploy", false, "Create the Pages project without starting deployment. Provider upload_v2 still requires --resource-id.")

	pagesDeployCmd.Flags().StringVar(&pagesResourceID, "resource-id", "", "ProjectDeployResourceID returned by `pages upload`.")
	pagesDeployListCmd.Flags().IntVar(&pagesLimit, "limit", volcengineDefaultListLimit, "Maximum number of Pages deployments to return (1-100).")
	pagesDeployListCmd.Flags().IntVar(&pagesOffset, "offset", 0, "Number of Pages deployments to skip. Must be a multiple of --limit.")
	pagesDeployCmd.AddCommand(pagesDeployListCmd)

	pagesFastCreateFlags := pagesFastCreateCmd.Flags()
	pagesFastCreateFlags.StringVar(&pagesFastFilePath, "file-path", "", "Path to the frontend directory or zip archive to upload.")
	pagesFastCreateFlags.StringVar(&pagesFunctionsInit, "functions-init", "", "Path to the backend root directory. Edge Functions are loaded from <backend>/functions/<slug>.")
	pagesFastCreateFlags.StringVar(&pagesMigrationsInit, "migrations-init", "", "Path to a directory containing SQL migration files to execute before binding Pages.")
	pagesFastCreateFlags.StringVar(&pagesCustomPrefix, "custom-prefix", "", "Environment variable prefix for binding a Pages project already used by another Supabase branch.")
	pagesFastCreateFlags.StringVar(&pagesFramePrefix, "framework-prefix", "", "Framework client prefix for browser-exposed Supabase env vars.")
	pagesFastCmd.AddCommand(pagesFastCreateCmd)

	pagesCmd.AddCommand(pagesListCmd)
	pagesCmd.AddCommand(pagesEnvVarsCmd)
	pagesCmd.AddCommand(pagesBindingCmd)
	pagesCmd.AddCommand(pagesBindCmd)
	pagesCmd.AddCommand(pagesCreateCmd)
	pagesCmd.AddCommand(pagesUploadCmd)
	pagesCmd.AddCommand(pagesDeployCmd)
	pagesCmd.AddCommand(pagesFastCmd)
	pagesCmd.AddCommand(pagesSyncCmd)
	pagesCmd.AddCommand(pagesUnbindCmd)
	rootCmd.AddCommand(pagesCmd)
}

func resolvePagesWorkspace(cmd *cobra.Command, title string) error {
	if flags.ProjectRef == "" {
		return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), title)
	}
	return ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false)
}

func addPagesWorkspaceFlags(cmd *cobra.Command, workspaceHelp string) {
	commandFlags := cmd.Flags()
	commandFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id.")
	markFlagTelemetrySafe(commandFlags.Lookup("project-ref"))
	commandFlags.StringVar(&flags.ProjectRef, "workspace-id", "", workspaceHelp)
	markFlagTelemetrySafe(commandFlags.Lookup("workspace-id"))
}
