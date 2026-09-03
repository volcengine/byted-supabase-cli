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

type pagesOptions struct {
	WorkspaceID string
	BranchID    string
}

func pagesCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newPagesCommand}}
}

func newPagesCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "pages",
		Short:   "Manage Volcengine IGA Pages Integrations for Supabase",
	}
	cmd.AddCommand(
		newPagesListCommand(ctx),
		newPagesEnvVarsCommand(ctx),
		newPagesBindingCommand(ctx),
		newPagesBindCommand(ctx),
		newPagesCreateCommand(ctx),
		newPagesUploadCommand(ctx),
		newPagesDeployCommand(ctx),
		newPagesFastCommand(ctx),
		newPagesSyncCommand(ctx),
		newPagesUnbindCommand(ctx),
	)
	return cmd
}

func addPagesWorkspaceFlagsFactory(cmd *cobra.Command, options *pagesOptions, help string) {
	flags := cmd.Flags()
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", help)
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func resolvePagesWorkspaceFactory(cmd *cobra.Command, options *pagesOptions, title string) error {
	previous := flags.ProjectRef
	if options.WorkspaceID != "" {
		flags.ProjectRef = options.WorkspaceID
	}
	err := loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), title)
	if err == nil {
		options.WorkspaceID = flags.ProjectRef
	}
	flags.ProjectRef = previous
	return err
}

func newPagesListCommand(ctx RuntimeContext) *cobra.Command {
	var options pagesOptions
	var name string
	var limit, offset int
	cmd := &cobra.Command{
		Use: "list", Short: "List IGA Pages projects", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(limit, offset, false); err != nil {
				return err
			}
			if offset%limit != 0 {
				return errors.New("--offset must be a multiple of --limit for Pages pagination")
			}
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			return pages.List(cmd.Context(), pages.ListParams{Name: name, Limit: limit, Offset: offset, WorkspaceID: options.WorkspaceID, BranchID: options.BranchID})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Pages project name filter.")
	cmd.Flags().IntVar(&limit, "limit", volcengineDefaultListLimit, "Maximum number of Pages projects to return (1-100).")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of Pages projects to skip. Must be a multiple of --limit.")
	addPagesWorkspaceFlagsFactory(cmd, &options, "Optional Supabase workspace filter. Does not default to linked project.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Optional Supabase branch filter.")
	_ = ctx
	return cmd
}

func newPagesEnvVarsCommand(ctx RuntimeContext) *cobra.Command {
	var options pagesOptions
	var showValues bool
	cmd := newPagesScopedCommand("env-vars", "List Supabase environment variables available for IGA Pages", ctx, &options, "Which project do you want to list Pages env vars for?", func(cmd *cobra.Command) error {
		return pages.EnvVars(cmd.Context(), pages.ScopedParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID, ShowValues: showValues})
	})
	addPagesWorkspaceFlagsFactory(cmd, &options, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	cmd.Flags().BoolVar(&showValues, "show-values", false, "Show sensitive environment variable values. By default values are masked.")
	return cmd
}

func newPagesBindingCommand(ctx RuntimeContext) *cobra.Command {
	var options pagesOptions
	cmd := newPagesScopedCommand("binding", "Show the IGA Pages binding for a Supabase branch", ctx, &options, "Which project do you want to show Pages binding for?", func(cmd *cobra.Command) error {
		return pages.Binding(cmd.Context(), pages.ScopedParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID})
	})
	addPagesWorkspaceFlagsFactory(cmd, &options, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	return cmd
}

func newPagesBindCommand(ctx RuntimeContext) *cobra.Command {
	var options pagesOptions
	var customPrefix, frameworkPrefix string
	cmd := &cobra.Command{
		Use:   "bind <pages-project-id>",
		Short: "Bind an IGA Pages project to a Supabase branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolvePagesWorkspaceFactory(cmd, &options, "Which project do you want to bind Pages for?"); err != nil {
				return err
			}
			return pages.Bind(cmd.Context(), pages.BindParams{
				WorkspaceID: options.WorkspaceID, BranchID: options.BranchID,
				PagesProjectID: args[0], CustomPrefix: customPrefix, FrameworkPrefix: frameworkPrefix,
			})
		},
	}
	addPagesWorkspaceFlagsFactory(cmd, &options, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID to bind. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&customPrefix, "custom-prefix", "", "Environment variable prefix for binding a Pages project already used by another Supabase branch.")
	cmd.Flags().StringVar(&frameworkPrefix, "framework-prefix", "", "Framework client prefix for browser-exposed Supabase env vars.")
	return cmd
}

func newPagesScopedCommand(use, short string, ctx RuntimeContext, options *pagesOptions, title string, run func(*cobra.Command) error) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := resolvePagesWorkspaceFactory(cmd, options, title); err != nil {
			return err
		}
		return run(cmd)
	}}
}

func newPagesCreateCommand(ctx RuntimeContext) *cobra.Command {
	var provider, scope, resourceID, framework, rootDir, outputDir, buildCmd, installCmd, nodejsVersion string
	var noDeploy bool
	cmd := &cobra.Command{Use: "create <project-name>", Short: "Create an IGA Pages project", Long: "Create an IGA Pages project. Project name must contain only lowercase letters, numbers, and hyphens.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return pages.Create(cmd.Context(), pages.CreateParams{Name: args[0], Provider: provider, Scope: scope, ProjectDeployResourceID: resourceID, Framework: framework, RootDir: rootDir, OutputDir: outputDir, BuildCmd: buildCmd, InstallCmd: installCmd, NodejsVersion: nodejsVersion, NoDeploy: noDeploy})
	}}
	cmd.Flags().StringVar(&provider, "provider", "upload_v2", "Pages project provider. Currently only upload_v2 is supported.")
	cmd.Flags().StringVar(&scope, "scope", "domestic", "Pages project scope: domestic, overseas, or global.")
	cmd.Flags().StringVar(&resourceID, "resource-id", "", "ProjectDeployResourceID returned by `pages upload`.")
	cmd.Flags().StringVar(&framework, "framework", "", "Pages project framework.")
	cmd.Flags().StringVar(&rootDir, "root-dir", "", "Pages project root directory.")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Pages project output directory.")
	cmd.Flags().StringVar(&buildCmd, "build-cmd", "", "Pages project build command.")
	cmd.Flags().StringVar(&installCmd, "install-cmd", "", "Pages project install command.")
	cmd.Flags().StringVar(&nodejsVersion, "nodejs-version", "", "Pages project Node.js version.")
	cmd.Flags().BoolVar(&noDeploy, "no-deploy", false, "Create the Pages project without starting deployment. Provider upload_v2 still requires --resource-id.")
	_ = ctx
	return cmd
}

func newPagesUploadCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "upload <file>", Short: "Upload an IGA Pages deployment resource", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return pages.Upload(cmd.Context(), pages.UploadParams{FilePath: args[0]})
	}}
	_ = ctx
	return cmd
}

func newPagesDeployCommand(ctx RuntimeContext) *cobra.Command {
	var resourceID string
	cmd := &cobra.Command{Use: "deploy <pages-project-id>", Short: "Create an IGA Pages deployment", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return pages.Deploy(cmd.Context(), pages.DeployParams{PagesProjectID: args[0], ProjectDeployResourceID: resourceID})
	}}
	cmd.Flags().StringVar(&resourceID, "resource-id", "", "ProjectDeployResourceID returned by `pages upload`.")
	deployList := newPagesDeployListCommand(ctx)
	cmd.AddCommand(deployList)
	return cmd
}

func newPagesDeployListCommand(ctx RuntimeContext) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{Use: "list <pages-project-id>", Short: "List IGA Pages deployments", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateVolcenginePageFlags(limit, offset, false); err != nil {
			return err
		}
		if offset%limit != 0 {
			return errors.New("--offset must be a multiple of --limit for Pages deploy pagination")
		}
		return pages.DeployList(cmd.Context(), pages.DeployListParams{PagesProjectID: args[0], Limit: limit, Offset: offset})
	}}
	cmd.Flags().IntVar(&limit, "limit", volcengineDefaultListLimit, "Maximum number of Pages deployments to return (1-100).")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of Pages deployments to skip. Must be a multiple of --limit.")
	_ = ctx
	return cmd
}

func newPagesFastCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "fast", Short: "Run opinionated Pages workflows"}
	cmd.AddCommand(newPagesFastCreateCommand(ctx))
	return cmd
}

func newPagesFastCreateCommand(ctx RuntimeContext) *cobra.Command {
	var filePath, functionsInit, migrationsInit, customPrefix, frameworkPrefix string
	cmd := &cobra.Command{Use: "create <project-name>", Short: "Create a new Pages project and Supabase workspace from a frontend directory or archive", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
			return err
		}
		return pages.FastCreate(cmd.Context(), pages.FastCreateParams{ProjectName: args[0], FilePath: filePath, CustomPrefix: customPrefix, FrameworkPrefix: frameworkPrefix, FunctionsInit: functionsInit, MigrationsInit: migrationsInit})
	}}
	cmd.Flags().StringVar(&filePath, "file-path", "", "Path to the frontend directory or zip archive to upload.")
	cmd.Flags().StringVar(&functionsInit, "functions-init", "", "Path to the backend root directory. Edge Functions are loaded from <backend>/functions/<slug>.")
	cmd.Flags().StringVar(&migrationsInit, "migrations-init", "", "Path to a directory containing SQL migration files to execute before binding Pages.")
	cmd.Flags().StringVar(&customPrefix, "custom-prefix", "", "Environment variable prefix for binding a Pages project already used by another Supabase branch.")
	cmd.Flags().StringVar(&frameworkPrefix, "framework-prefix", "", "Framework client prefix for browser-exposed Supabase env vars.")
	_ = ctx
	return cmd
}

func newPagesSyncCommand(ctx RuntimeContext) *cobra.Command {
	var options pagesOptions
	cmd := newPagesScopedCommand("sync", "Sync Supabase environment variables to the bound IGA Pages project", ctx, &options, "Which project do you want to sync Pages env vars for?", func(cmd *cobra.Command) error {
		return pages.Sync(cmd.Context(), pages.SyncParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID})
	})
	addPagesWorkspaceFlagsFactory(cmd, &options, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID to sync. Defaults to the workspace default branch.")
	return cmd
}

func newPagesUnbindCommand(ctx RuntimeContext) *cobra.Command {
	var options pagesOptions
	cmd := newPagesScopedCommand("unbind", "Unbind IGA Pages from a Supabase branch", ctx, &options, "Which project do you want to unbind Pages from?", func(cmd *cobra.Command) error {
		return pages.Unbind(cmd.Context(), pages.ScopedParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID})
	})
	addPagesWorkspaceFlagsFactory(cmd, &options, "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID to unbind. Defaults to the workspace default branch.")
	return cmd
}
