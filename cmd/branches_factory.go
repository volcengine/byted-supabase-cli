// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/branches/create"
	branchdelete "github.com/volcengine/byted-supabase-cli/internal/branches/delete"
	"github.com/volcengine/byted-supabase-cli/internal/branches/get"
	"github.com/volcengine/byted-supabase-cli/internal/branches/list"
	"github.com/volcengine/byted-supabase-cli/internal/branches/manage"
	branchupdate "github.com/volcengine/byted-supabase-cli/internal/branches/update"
	"github.com/volcengine/byted-supabase-cli/internal/errdefs"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type branchesOptions struct {
	WorkspaceID string
}

func newBranchesCommandFactory(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "branches",
		Short:   "Manage Volcengine Supabase Preview Branches",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newBranchesCreateCommand(ctx),
		newBranchesListCommand(ctx),
		newBranchesGetCommand(ctx),
		newBranchesUpdateCommand(ctx),
		newBranchesDeleteCommand(ctx),
		newBranchesGetDefaultCommand(ctx),
		newBranchesSetDefaultCommand(ctx),
		newBranchesRestartCommand(ctx),
		newBranchesRestoreWindowCommand(ctx),
		newBranchesRestorableCommand(ctx),
		newBranchesRestoreBranchCommand(ctx),
		newBranchesStudioLoginCommand(ctx),
	)
	return cmd
}

func newBranchesCommand(ctx RuntimeContext) *cobra.Command {
	return newBranchesCommandFactory(ctx)
}

func addBranchesWorkspaceFlags(cmd *cobra.Command, options *branchesOptions) {
	flags := cmd.Flags()
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func prepareBranchesWorkspace(cmd *cobra.Command, fsys afero.Fs, options *branchesOptions, prompt string) error {
	previous := flags.ProjectRef
	if options.WorkspaceID != "" {
		flags.ProjectRef = options.WorkspaceID
	}
	err := loadOrPromptVolcengineProjectRef(cmd.Context(), fsys, prompt)
	if err == nil {
		options.WorkspaceID = flags.ProjectRef
	}
	flags.ProjectRef = previous
	return err
}

func newBranchesCreateCommand(ctx RuntimeContext) *cobra.Command {
	options := struct {
		branchesOptions
		ParentID   string
		ParentTime string
	}{}
	cmd := &cobra.Command{
		Use: "create [name]", Short: "Create a preview branch",
		Long: "Create a preview branch for the linked project.", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareBranchesWorkspace(cmd, ctx.FS, &options.branchesOptions, "Which project do you want to create a branch for?"); err != nil {
				return err
			}
			name := ""
			if len(args) > 0 {
				name = strings.TrimSpace(args[0])
			} else {
				var err error
				name, err = promptVolcengineBranchName(cmd.Context())
				if err != nil {
					return err
				}
			}
			if name == "" {
				return errors.New("branch name cannot be empty")
			}
			options.ParentID = strings.TrimSpace(options.ParentID)
			options.ParentTime = strings.TrimSpace(options.ParentTime)
			if options.ParentTime != "" {
				if _, err := time.Parse(time.RFC3339, options.ParentTime); err != nil {
					return errors.Errorf("invalid --parent-time %q, expected RFC3339 format like 2026-05-20T10:00:00Z", options.ParentTime)
				}
			}
			return create.RunVolcengine(cmd.Context(), create.RunVolcengineParams{
				WorkspaceID: options.WorkspaceID, Name: name, ParentID: options.ParentID, ParentTime: options.ParentTime,
			})
		},
	}
	cmd.Flags().StringVar(&options.ParentID, "parent-id", "", "Parent branch ID to create from. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&options.ParentTime, "parent-time", "", "Parent branch restore time in RFC3339 format, for example 2026-05-20T10:00:00Z.")
	addBranchesWorkspaceFlags(cmd, &options.branchesOptions)
	return cmd
}

func newBranchesListCommand(ctx RuntimeContext) *cobra.Command {
	options := struct {
		branchesOptions
		Search   string
		ParentID string
		Limit    int
		Offset   int
	}{}
	cmd := &cobra.Command{
		Use: "list", Short: "List all preview branches", Long: "List all preview branches of the linked project.", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(options.Limit, options.Offset, true); err != nil {
				return err
			}
			if err := requireBranchesListWorkspace(ctx.FS, &options.branchesOptions); err != nil {
				return err
			}
			if err := prepareBranchesWorkspace(cmd, ctx.FS, &options.branchesOptions, "Which project do you want to list branches for?"); err != nil {
				return err
			}
			return list.RunVolcengine(cmd.Context(), list.RunVolcengineParams{
				WorkspaceID: options.WorkspaceID, Search: options.Search, ParentID: options.ParentID, Limit: options.Limit, Offset: options.Offset,
			})
		},
	}
	cmd.Flags().StringVar(&options.Search, "search", "", "Search branches by keyword.")
	cmd.Flags().StringVar(&options.ParentID, "parent-id", "", "List direct child branches of the specified parent branch ID.")
	cmd.Flags().IntVar(&options.Limit, "limit", volcengineDefaultListLimit, "Maximum number of branches to return (1-100). Set 0 to list all.")
	cmd.Flags().IntVar(&options.Offset, "offset", 0, "Number of branches to skip before returning results.")
	addBranchesWorkspaceFlags(cmd, &options.branchesOptions)
	return cmd
}

func requireBranchesListWorkspace(fsys afero.Fs, options *branchesOptions) error {
	if options.WorkspaceID != "" || flags.ProjectRef != "" {
		return nil
	}
	linkedWorkspaceID, err := volcengine.LoadLinkedWorkspaceID(fsys)
	if err != nil {
		return err
	}
	if linkedWorkspaceID == "" {
		return errdefs.ErrMissingWorkspaceID
	}
	return nil
}

func newBranchesGetCommand(ctx RuntimeContext) *cobra.Command {
	options := branchesOptions{}
	var branchID string
	cmd := newBranchesTargetCommand("get [branch-id]", "Retrieve details of a preview branch", &options, ctx, &branchID, "Which project do you want to get branch details for?", func(cmd *cobra.Command) error {
		return get.RunVolcengine(cmd.Context(), get.RunVolcengineParams{WorkspaceID: options.WorkspaceID, BranchID: branchID})
	})
	return cmd
}

func newBranchesUpdateCommand(ctx RuntimeContext) *cobra.Command {
	options := branchesOptions{}
	var branchID, name string
	cmd := newBranchesTargetCommand("update [branch-id]", "Update a preview branch", &options, ctx, &branchID, "Which project do you want to update a branch for?", func(cmd *cobra.Command) error {
		if strings.TrimSpace(name) == "" {
			var updatedName *string
			if err := promptVolcengineBranchUpdateName(cmd.Context(), &updatedName); err != nil {
				return err
			}
			if updatedName == nil {
				return errors.New("branch name cannot be empty")
			}
			name = *updatedName
		}
		name = strings.TrimSpace(name)
		return branchupdate.RunVolcengine(cmd.Context(), branchupdate.RunVolcengineParams{WorkspaceID: options.WorkspaceID, BranchID: branchID, Name: &name})
	})
	cmd.Flags().StringVar(&name, "name", "", "Rename the preview branch.")
	return cmd
}

func newBranchesDeleteCommand(ctx RuntimeContext) *cobra.Command {
	options := branchesOptions{}
	var branchID string
	return newBranchesTargetCommand("delete [branch-id]", "Delete a preview branch", &options, ctx, &branchID, "Which project do you want to delete a branch from?", func(cmd *cobra.Command) error {
		return branchdelete.RunVolcengine(cmd.Context(), branchdelete.RunVolcengineParams{WorkspaceID: options.WorkspaceID, BranchID: branchID})
	})
}

func newBranchesTargetCommand(use, short string, parent *branchesOptions, ctx RuntimeContext, branchID *string, prompt string, run func(*cobra.Command) error) *cobra.Command {
	cmd := &cobra.Command{
		Use: use, Short: short, Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareBranchesWorkspace(cmd, ctx.FS, parent, prompt); err != nil {
				return err
			}
			if len(args) > 0 {
				*branchID = strings.TrimSpace(args[0])
			} else if err := promptVolcengineBranchID(cmd.Context(), parent.WorkspaceID, branchID); err != nil {
				return err
			}
			return run(cmd)
		},
	}
	addBranchesWorkspaceFlags(cmd, parent)
	return cmd
}

func newBranchesGetDefaultCommand(ctx RuntimeContext) *cobra.Command {
	options := branchesOptions{}
	cmd := &cobra.Command{Use: "get-default", Short: "Retrieve the default preview branch", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := prepareBranchesWorkspace(cmd, ctx.FS, &options, "Which project do you want to get the default branch for?"); err != nil {
			return err
		}
		return manage.RunGetDefault(cmd.Context(), manage.GetDefaultParams{WorkspaceID: options.WorkspaceID})
	}}
	addBranchesWorkspaceFlags(cmd, &options)
	return cmd
}

func newBranchesSetDefaultCommand(ctx RuntimeContext) *cobra.Command {
	options := branchesOptions{}
	var branchID string
	return newBranchesTargetCommand("set-default [branch-id]", "Set a preview branch as default", &options, ctx, &branchID, "Which project do you want to set the default branch for?", func(cmd *cobra.Command) error {
		return manage.RunSetDefault(cmd.Context(), manage.BranchActionParams{WorkspaceID: options.WorkspaceID, BranchID: branchID})
	})
}

func newBranchesRestartCommand(ctx RuntimeContext) *cobra.Command {
	parent := &branchesOptions{}
	var options struct{ ComputeIDs []string }
	var branchID string
	cmd := newBranchesTargetCommand("restart [branch-id]", "Restart branch compute", parent, ctx, &branchID, "Which project do you want to restart a branch for?", func(cmd *cobra.Command) error {
		return manage.RunRestart(cmd.Context(), manage.BranchActionParams{WorkspaceID: parent.WorkspaceID, BranchID: branchID, ComputeIDs: options.ComputeIDs})
	})
	cmd.Flags().StringArrayVar(&options.ComputeIDs, "compute-id", nil, "Compute ID to restart. Can be specified multiple times; defaults to the whole branch.")
	return cmd
}

func newBranchesRestoreWindowCommand(ctx RuntimeContext) *cobra.Command {
	parent := &branchesOptions{}
	var branchID string
	return newBranchesTargetCommand("restore-window [branch-id]", "Retrieve the restore window of a preview branch", parent, ctx, &branchID, "Which project do you want to get the restore window for?", func(cmd *cobra.Command) error {
		return manage.RunRestoreWindow(cmd.Context(), manage.BranchActionParams{WorkspaceID: parent.WorkspaceID, BranchID: branchID})
	})
}

func newBranchesRestorableCommand(ctx RuntimeContext) *cobra.Command {
	parent := &branchesOptions{}
	var options struct {
		RestoreTime, Search string
		Limit, Offset       int
	}
	cmd := &cobra.Command{Use: "restorable", Short: "List branches restorable at a target time", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateVolcenginePageFlags(options.Limit, options.Offset, false); err != nil {
			return err
		}
		if err := prepareBranchesWorkspace(cmd, ctx.FS, parent, "Which project do you want to list restorable branches for?"); err != nil {
			return err
		}
		if err := ensureVolcengineRestoreTime(cmd.Context(), &options.RestoreTime); err != nil {
			return err
		}
		return manage.RunRestorable(cmd.Context(), manage.BranchActionParams{WorkspaceID: parent.WorkspaceID, RestoreTime: options.RestoreTime, Search: options.Search, Limit: options.Limit, Offset: options.Offset})
	}}
	cmd.Flags().StringVar(&options.RestoreTime, "restore-time", "", "Target restore time in RFC3339 format, for example 2026-05-20T10:00:00Z.")
	cmd.Flags().StringVar(&options.Search, "search", "", "Search restorable branches by branch ID or name prefix.")
	cmd.Flags().IntVar(&options.Limit, "limit", volcengineDefaultListLimit, "Maximum number of restorable branches to return (1-100).")
	cmd.Flags().IntVar(&options.Offset, "offset", 0, "Number of restorable branches to skip before returning results.")
	addBranchesWorkspaceFlags(cmd, parent)
	return cmd
}

func newBranchesRestoreBranchCommand(ctx RuntimeContext) *cobra.Command {
	parent := &branchesOptions{}
	var options struct{ RestoreTime, SourceBranchID string }
	var branchID string
	cmd := newBranchesTargetCommand("restore [branch-id]", "Restore a preview branch to a target time", parent, ctx, &branchID, "Which project do you want to restore a branch for?", func(cmd *cobra.Command) error {
		if err := ensureVolcengineRestoreTime(cmd.Context(), &options.RestoreTime); err != nil {
			return err
		}
		return manage.RunRestore(cmd.Context(), manage.BranchActionParams{WorkspaceID: parent.WorkspaceID, BranchID: branchID, RestoreTime: options.RestoreTime, SourceBranchID: strings.TrimSpace(options.SourceBranchID)})
	})
	cmd.Flags().StringVar(&options.RestoreTime, "restore-time", "", "Target restore time in RFC3339 format, for example 2026-05-20T10:00:00Z.")
	cmd.Flags().StringVar(&options.SourceBranchID, "source-branch-id", "", "Source branch ID to restore from. Defaults to the target branch.")
	return cmd
}

func newBranchesStudioLoginCommand(ctx RuntimeContext) *cobra.Command {
	parent := &branchesOptions{}
	var username, password, branchID string
	cmd := newBranchesTargetCommand("update-studio-login [branch-id]", "Update Studio login credentials for a branch", parent, ctx, &branchID, "Which project do you want to update Studio login for?", func(cmd *cobra.Command) error {
		username, err := ensureStudioUsername(cmd.Context(), username)
		if err != nil {
			return err
		}
		password, err := ensureStudioPassword(password)
		if err != nil {
			return err
		}
		return manage.RunUpdateStudioLogin(cmd.Context(), manage.UpdateStudioLoginParams{WorkspaceID: parent.WorkspaceID, BranchID: branchID, Username: username, Password: password})
	})
	cmd.Flags().StringVar(&username, "username", "", "Studio login username.")
	cmd.Flags().StringVar(&password, "password", "", "Studio login password.")
	return cmd
}
