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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/branches/create"
	"github.com/volcengine/byted-supabase-cli/internal/branches/delete"
	"github.com/volcengine/byted-supabase-cli/internal/branches/disable"
	"github.com/volcengine/byted-supabase-cli/internal/branches/get"
	"github.com/volcengine/byted-supabase-cli/internal/branches/list"
	"github.com/volcengine/byted-supabase-cli/internal/branches/manage"
	"github.com/volcengine/byted-supabase-cli/internal/branches/pause"
	"github.com/volcengine/byted-supabase-cli/internal/branches/unpause"
	"github.com/volcengine/byted-supabase-cli/internal/branches/update"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
	"golang.org/x/term"
)

var (
	branchesCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "branches",
		Short:   "Manage Supabase preview branches",
	}

	persistent           bool
	withData             bool
	notifyURL            string
	branchSearch         string
	branchParentID       string
	branchParentTime     string
	branchComputeIDs     []string
	branchRestoreTime    string
	branchSourceBranchID string
	branchStudioUsername string
	branchStudioPassword string
	branchListLimit      int
	branchListOffset     int
	branchRestoreLimit   int
	branchRestoreOffset  int

	branchCreateCmd = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a preview branch",
		Long:  "Create a preview branch for the linked project.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to create a branch for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			var name string
			if len(args) > 0 {
				name = strings.TrimSpace(args[0])
			} else if value, err := promptVolcengineBranchName(ctx); err != nil {
				return err
			} else {
				name = value
			}
			if name == "" {
				return errors.New("branch name cannot be empty")
			}
			branchParentID = strings.TrimSpace(branchParentID)
			branchParentTime = strings.TrimSpace(branchParentTime)
			if branchParentTime != "" {
				if _, err := time.Parse(time.RFC3339, branchParentTime); err != nil {
					return errors.Errorf("invalid --parent-time %q, expected RFC3339 format like 2026-05-20T10:00:00Z", branchParentTime)
				}
			}
			return create.RunVolcengine(ctx, create.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				Name:        name,
				ParentID:    branchParentID,
				ParentTime:  branchParentTime,
			})

			// Original Supabase implementation:
			// body := api.CreateBranchBody{IsDefault: cast.Ptr(false)}
			// if len(args) > 0 {
			// 	body.BranchName = args[0]
			// }
			// cmdFlags := cmd.Flags()
			// if cmdFlags.Changed("region") {
			// 	body.Region = &region.Value
			// }
			// if cmdFlags.Changed("size") {
			// 	body.DesiredInstanceSize = (*api.CreateBranchBodyDesiredInstanceSize)(&size.Value)
			// }
			// if cmdFlags.Changed("persistent") {
			// 	body.Persistent = &persistent
			// }
			// if cmdFlags.Changed("with-data") {
			// 	body.WithData = &withData
			// }
			// if cmdFlags.Changed("notify-url") {
			// 	body.NotifyUrl = &notifyURL
			// }
			// return create.Run(cmd.Context(), body, afero.NewOsFs())
		},
	}

	branchListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all preview branches",
		Long:  "List all preview branches of the linked project.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(branchListLimit, branchListOffset, true); err != nil {
				return err
			}
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to list branches for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return list.RunVolcengine(ctx, list.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				Search:      branchSearch,
				ParentID:    branchParentID,
				Limit:       branchListLimit,
				Offset:      branchListOffset,
			})

			// Original Supabase implementation:
			// return list.Run(cmd.Context(), afero.NewOsFs())
		},
	}

	branchId string

	branchGetCmd = &cobra.Command{
		Use:   "get [branch-id]",
		Short: "Retrieve details of a preview branch",
		Long:  "Retrieve details of the specified preview branch.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to get branch details for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			return get.RunVolcengine(ctx, get.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
			})

			// Original Supabase implementation:
			// if len(args) > 0 {
			// 	branchId = args[0]
			// } else if err := promptBranchId(ctx, fsys); err != nil {
			// 	return err
			// }
			// return get.Run(ctx, branchId, fsys)
		},
	}

	branchStatus = utils.EnumFlag{
		Allowed: []string{
			string(api.BranchResponseStatusRUNNINGMIGRATIONS),
			string(api.BranchResponseStatusMIGRATIONSPASSED),
			string(api.BranchResponseStatusMIGRATIONSFAILED),
			string(api.BranchResponseStatusFUNCTIONSDEPLOYED),
			string(api.BranchResponseStatusFUNCTIONSFAILED),
		},
	}
	branchName string
	gitBranch  string

	branchUpdateCmd = &cobra.Command{
		Use:   "update [branch-id]",
		Short: "Update a preview branch",
		Long:  "Update a preview branch by its ID.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to update a branch for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}

			cmdFlags := cmd.Flags()
			nameChanged := cmdFlags.Changed("name")

			var name *string
			if nameChanged {
				branchName = strings.TrimSpace(branchName)
				if branchName == "" {
					return errors.New("branch name cannot be empty")
				}
				name = &branchName
			}
			if !nameChanged {
				if err := promptVolcengineBranchUpdateName(ctx, &name); err != nil {
					return err
				}
			}

			return update.RunVolcengine(ctx, update.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
				Name:        name,
			})

			// Original Supabase implementation:
			// cmdFlags := cmd.Flags()
			// var body api.UpdateBranchBody
			// if cmdFlags.Changed("name") {
			// 	body.BranchName = &branchName
			// }
			// if cmdFlags.Changed("git-branch") {
			// 	body.GitBranch = &gitBranch
			// }
			// if cmdFlags.Changed("persistent") {
			// 	body.Persistent = &persistent
			// }
			// if cmdFlags.Changed("status") {
			// 	body.Status = (*api.UpdateBranchBodyStatus)(&branchStatus.Value)
			// }
			// if cmdFlags.Changed("notify-url") {
			// 	body.NotifyUrl = &notifyURL
			// }
			// if len(args) > 0 {
			// 	branchId = args[0]
			// } else if err := promptBranchId(ctx, fsys); err != nil {
			// 	return err
			// }
			// return update.Run(cmd.Context(), branchId, body, fsys)
		},
	}

	branchPauseCmd = &cobra.Command{
		Use:   "pause [name]",
		Short: "Pause a preview branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptBranchId(ctx, fsys); err != nil {
				return err
			}
			return pause.Run(ctx, branchId)
		},
	}

	branchUnpauseCmd = &cobra.Command{
		Use:   "unpause [name]",
		Short: "Unpause a preview branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptBranchId(ctx, fsys); err != nil {
				return err
			}
			return unpause.Run(ctx, branchId)
		},
	}

	branchDeleteCmd = &cobra.Command{
		Use:   "delete [branch-id]",
		Short: "Delete a preview branch",
		Long:  "Delete a preview branch by its ID.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to delete a branch from?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			return delete.RunVolcengine(ctx, delete.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
			})

			// Original Supabase implementation:
			// if len(args) > 0 {
			// 	branchId = args[0]
			// } else if err := promptBranchId(ctx, fsys); err != nil {
			// 	return err
			// }
			// return delete.Run(ctx, branchId, nil)
		},
	}

	branchGetDefaultCmd = &cobra.Command{
		Use:   "get-default",
		Short: "Retrieve the default preview branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to get the default branch for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return manage.RunGetDefault(ctx, manage.GetDefaultParams{
				WorkspaceID: flags.ProjectRef,
			})
		},
	}

	branchSetDefaultCmd = &cobra.Command{
		Use:   "set-default [branch-id]",
		Short: "Set a preview branch as default",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to set the default branch for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			return manage.RunSetDefault(ctx, manage.BranchActionParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
			})
		},
	}

	branchRestartCmd = &cobra.Command{
		Use:   "restart [branch-id]",
		Short: "Restart a preview branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to restart a branch for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			return manage.RunRestart(ctx, manage.BranchActionParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
				ComputeIDs:  branchComputeIDs,
			})
		},
	}

	branchRestoreWindowCmd = &cobra.Command{
		Use:   "restore-window [branch-id]",
		Short: "Retrieve the restore window of a preview branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to get the restore window for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			return manage.RunRestoreWindow(ctx, manage.BranchActionParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
			})
		},
	}

	branchRestorableCmd = &cobra.Command{
		Use:   "restorable",
		Short: "List branches restorable at a target time",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(branchRestoreLimit, branchRestoreOffset, false); err != nil {
				return err
			}
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to list restorable branches for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if err := ensureVolcengineRestoreTime(ctx, &branchRestoreTime); err != nil {
				return err
			}
			return manage.RunRestorable(ctx, manage.BranchActionParams{
				WorkspaceID: flags.ProjectRef,
				RestoreTime: branchRestoreTime,
				Search:      branchSearch,
				Limit:       branchRestoreLimit,
				Offset:      branchRestoreOffset,
			})
		},
	}

	branchRestoreCmd = &cobra.Command{
		Use:   "restore [branch-id]",
		Short: "Restore a preview branch to a target time",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to restore a branch for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = args[0]
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			if err := ensureVolcengineRestoreTime(ctx, &branchRestoreTime); err != nil {
				return err
			}
			return manage.RunRestore(ctx, manage.BranchActionParams{
				WorkspaceID:    flags.ProjectRef,
				BranchID:       branchId,
				RestoreTime:    branchRestoreTime,
				SourceBranchID: strings.TrimSpace(branchSourceBranchID),
			})
		},
	}

	branchUpdateStudioLoginCmd = &cobra.Command{
		Use:   "update-studio-login [branch-id]",
		Short: "Update Studio login credentials for a branch",
		Long:  "Update Studio login credentials for a branch.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to update Studio login for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if len(args) > 0 {
				branchId = strings.TrimSpace(args[0])
			} else if err := promptVolcengineBranchID(ctx, flags.ProjectRef); err != nil {
				return err
			}
			username, err := ensureStudioUsername(ctx, branchStudioUsername)
			if err != nil {
				return err
			}
			password, err := ensureStudioPassword(branchStudioPassword)
			if err != nil {
				return err
			}
			return manage.RunUpdateStudioLogin(ctx, manage.UpdateStudioLoginParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    branchId,
				Username:    username,
				Password:    password,
			})
		},
	}

	branchDisableCmd = &cobra.Command{
		Hidden: true,
		Use:    "disable",
		Short:  "Disable preview branching",
		Long:   "Disable preview branching for the linked project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return disable.Run(cmd.Context(), afero.NewOsFs())
		},
	}
)

func init() {
	branchFlags := branchesCmd.PersistentFlags()
	branchFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(branchFlags.Lookup("project-ref"))
	branchFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(branchFlags.Lookup("workspace-id"))
	listFlags := branchListCmd.Flags()
	listFlags.StringVar(&branchSearch, "search", "", "Search branches by keyword.")
	listFlags.StringVar(&branchParentID, "parent-id", "", "List direct child branches of the specified parent branch ID.")
	listFlags.IntVar(&branchListLimit, "limit", volcengineDefaultListLimit, "Maximum number of branches to return (1-100). Set 0 to list all.")
	listFlags.IntVar(&branchListOffset, "offset", 0, "Number of branches to skip before returning results.")
	createFlags := branchCreateCmd.Flags()
	createFlags.StringVar(&branchParentID, "parent-id", "", "Parent branch ID to create from. Defaults to the workspace default branch.")
	createFlags.StringVar(&branchParentTime, "parent-time", "", "Parent branch restore time in RFC3339 format, for example 2026-05-20T10:00:00Z.")
	// Original Supabase flags unsupported or semantically different for Volcengine CreateBranch:
	// createFlags.Var(&region, "region", "Select a region to deploy the branch database.")
	// createFlags.Var(&size, "size", "Select a desired instance size for the branch database.")
	// createFlags.BoolVar(&persistent, "persistent", false, "Whether to create a persistent branch.")
	// createFlags.BoolVar(&withData, "with-data", false, "Whether to clone production data to the branch database.")
	// createFlags.StringVar(&notifyURL, "notify-url", "", "URL to notify when branch is active healthy.")
	branchesCmd.AddCommand(branchCreateCmd)
	branchesCmd.AddCommand(branchListCmd)
	branchesCmd.AddCommand(branchGetCmd)
	updateFlags := branchUpdateCmd.Flags()
	updateFlags.StringVar(&branchName, "name", "", "Rename the preview branch.")
	// Original Supabase flags unsupported by Volcengine UpdateBranch:
	// updateFlags.StringVar(&gitBranch, "git-branch", "", "Change the associated git branch.")
	// updateFlags.BoolVar(&persistent, "persistent", false, "Switch between ephemeral and persistent branch.")
	// updateFlags.Var(&branchStatus, "status", "Override the current branch status.")
	// updateFlags.StringVar(&notifyURL, "notify-url", "", "URL to notify when branch is active healthy.")
	branchesCmd.AddCommand(branchUpdateCmd)
	branchesCmd.AddCommand(branchDeleteCmd)
	branchesCmd.AddCommand(branchGetDefaultCmd)
	branchesCmd.AddCommand(branchSetDefaultCmd)
	restartFlags := branchRestartCmd.Flags()
	restartFlags.StringArrayVar(&branchComputeIDs, "compute-id", nil, "Compute ID to restart. Can be specified multiple times; defaults to the whole branch.")
	branchesCmd.AddCommand(branchRestartCmd)
	branchesCmd.AddCommand(branchRestoreWindowCmd)
	restorableFlags := branchRestorableCmd.Flags()
	restorableFlags.StringVar(&branchRestoreTime, "restore-time", "", "Target restore time in RFC3339 format, for example 2026-05-20T10:00:00Z.")
	restorableFlags.StringVar(&branchSearch, "search", "", "Search restorable branches by branch ID or name prefix.")
	restorableFlags.IntVar(&branchRestoreLimit, "limit", volcengineDefaultListLimit, "Maximum number of restorable branches to return (1-100).")
	restorableFlags.IntVar(&branchRestoreOffset, "offset", 0, "Number of restorable branches to skip before returning results.")
	branchesCmd.AddCommand(branchRestorableCmd)
	restoreFlags := branchRestoreCmd.Flags()
	restoreFlags.StringVar(&branchRestoreTime, "restore-time", "", "Target restore time in RFC3339 format, for example 2026-05-20T10:00:00Z.")
	restoreFlags.StringVar(&branchSourceBranchID, "source-branch-id", "", "Source branch ID to restore from. Defaults to the target branch.")
	branchesCmd.AddCommand(branchRestoreCmd)
	studioLoginFlags := branchUpdateStudioLoginCmd.Flags()
	studioLoginFlags.StringVar(&branchStudioUsername, "username", "", "Studio login username.")
	studioLoginFlags.StringVar(&branchStudioPassword, "password", "", "Studio login password.")
	branchesCmd.AddCommand(branchUpdateStudioLoginCmd)
	// Original Supabase branch lifecycle commands currently have no public Volcengine SDK equivalent.
	// Keep the command implementations for future reference, but do not register them in the Volcengine CLI.
	// branchesCmd.AddCommand(branchDisableCmd)
	// branchesCmd.AddCommand(branchPauseCmd)
	// branchesCmd.AddCommand(branchUnpauseCmd)
	rootCmd.AddCommand(branchesCmd)
}

func promptBranchId(ctx context.Context, fsys afero.Fs) error {
	if console := utils.NewConsole(); !console.IsTTY {
		// Only read from stdin if the terminal is non-interactive
		title := "Enter the name of your branch"
		if branchId = utils.GetGitBranch(fsys); len(branchId) > 0 {
			title += fmt.Sprintf(" (or leave blank to use %s)", utils.Aqua(branchId))
		}
		title += ": "
		if name, err := console.PromptText(ctx, title); err != nil {
			return err
		} else if len(name) > 0 {
			branchId = name
		}
		if len(branchId) == 0 {
			return errors.New("branch name cannot be empty")
		}
		return nil
	}
	branches, err := list.ListBranch(ctx, flags.ProjectRef)
	if err != nil {
		return err
	} else if len(branches) == 0 {
		utils.CmdSuggestion = fmt.Sprintf("Create your first branch with: %s", utils.Aqua("byted-supabase-cli branches create"))
		return errors.Errorf("branching is disabled")
	}
	// Let user choose from a list of branches
	items := make([]utils.PromptItem, len(branches))
	for i, branch := range branches {
		items[i] = utils.PromptItem{
			Summary: branch.Name,
			Details: branch.ProjectRef,
		}
	}
	title := "Select a branch:"
	choice, err := utils.PromptChoice(ctx, title, items)
	if err == nil {
		branchId = choice.Details
		fmt.Fprintln(os.Stderr, "Selected branch ID:", branchId)
	}
	return err
}

func promptVolcengineBranchName(ctx context.Context) (string, error) {
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing branch name. Supply branches create [name].")
	}
	name, err := console.PromptText(ctx, "Enter branch name: ")
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("branch name cannot be empty")
	}
	return name, nil
}

func promptVolcengineBranchID(ctx context.Context, workspaceID string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("missing branch id. Supply the branch ID argument.")
	}
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	var branches []volcengine.Branch
	for _, region := range cfgSet.Regions {
		result, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).DescribeAllBranches(ctx, volcengine.DescribeBranchesParams{
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return errors.Errorf("failed to list volcengine branches in region %s: %w", region, err)
		}
		branches = append(branches, result.Branches...)
	}
	if len(branches) == 0 {
		return errors.New("no volcengine branches found")
	}
	items := make([]utils.PromptItem, len(branches))
	for i, branch := range branches {
		items[i] = utils.PromptItem{
			Summary: branch.BranchID,
			Details: "name: " + branch.BranchName + ", status: " + branch.BranchStatus,
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a branch:", items)
	if err != nil {
		return err
	}
	branchId = choice.Summary
	fmt.Fprintln(os.Stderr, "Selected branch ID:", branchId)
	return nil
}

func promptVolcengineBranchUpdateName(ctx context.Context, name **string) error {
	console := utils.NewConsole()
	if !console.IsTTY {
		return errors.New("nothing to update. Supply --name.")
	}
	input, err := console.PromptText(ctx, "Enter new branch name: ")
	if err != nil {
		return err
	}
	if input = strings.TrimSpace(input); input != "" {
		*name = &input
		return nil
	}
	return errors.New("branch name cannot be empty")
}

func ensureVolcengineRestoreTime(ctx context.Context, value *string) error {
	*value = strings.TrimSpace(*value)
	if *value == "" {
		console := utils.NewConsole()
		if !console.IsTTY {
			return errors.New("missing restore time. Supply --restore-time in RFC3339 format.")
		}
		input, err := console.PromptText(ctx, "Enter restore time in RFC3339 format: ")
		if err != nil {
			return err
		}
		*value = strings.TrimSpace(input)
	}
	if *value == "" {
		return errors.New("restore time cannot be empty")
	}
	if _, err := time.Parse(time.RFC3339, *value); err != nil {
		return errors.Errorf("invalid --restore-time %q, expected RFC3339 format like 2026-05-20T10:00:00Z", *value)
	}
	return nil
}

func ensureStudioUsername(ctx context.Context, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" {
		return value, nil
	}
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing required flag: --username")
	}
	value, err := console.PromptText(ctx, "Enter Studio username: ")
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("Studio username cannot be empty")
	}
	return value, nil
}

func ensureStudioPassword(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("missing required flag: --password")
	}
	fmt.Fprint(os.Stderr, "Enter new Studio password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", errors.Errorf("failed to read Studio password: %w", err)
	}
	fmt.Fprint(os.Stderr, "Confirm new Studio password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", errors.Errorf("failed to read Studio password confirmation: %w", err)
	}
	if len(password) == 0 {
		return "", errors.New("Studio password cannot be empty")
	}
	if string(password) != string(confirmation) {
		return "", errors.New("Studio passwords do not match")
	}
	return string(password), nil
}
