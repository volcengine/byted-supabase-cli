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
	"strconv"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	operationlist "github.com/volcengine/byted-supabase-cli/internal/operations/list"
	"github.com/volcengine/byted-supabase-cli/internal/projects/apiKeys"
	"github.com/volcengine/byted-supabase-cli/internal/projects/create"
	"github.com/volcengine/byted-supabase-cli/internal/projects/delete"
	"github.com/volcengine/byted-supabase-cli/internal/projects/list"
	"github.com/volcengine/byted-supabase-cli/internal/projects/manage"
	"github.com/volcengine/byted-supabase-cli/internal/projects/overview"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
	"golang.org/x/term"
)

var (
	volcengineDefaultListLimit     = 10
	volcenginePromptWorkspaceLimit = 100

	projectsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "projects",
		Short:   "Manage Supabase projects",
	}

	interactive                  bool
	projectName                  string
	volcProjectName              string
	isAgentPlan                  bool
	agentPlanSeatID              string
	projectDetail                bool
	projectsListLimit            int
	projectsListOffset           int
	projectsOperationsBranchID   string
	projectsOperationsComputeID  string
	projectsOperationsActionName string
	projectsOperationsStatus     string
	projectsOperationsStartTime  string
	projectsOperationsEndTime    string
	projectsOperationsLimit      int
	projectsOperationsOffset     int
	volcRegion                   string
	volcBranchID                 string
	workspaceName                string
	tagValues                    []string
	tagKeys                      []string
	enablePolicy                 bool
	disablePolicy                bool
	minCU                        float64
	maxCU                        float64
	suspendTimeout               int
	computeSettingsServiceType   string
	retentionHours               int
	dbPassword                   string
	region                       = utils.EnumFlag{
		Allowed: utils.AwsRegions(),
	}
	size = utils.EnumFlag{
		Allowed: []string{
			string(api.V1CreateProjectBodyDesiredInstanceSizeLarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeMedium),
			string(api.V1CreateProjectBodyDesiredInstanceSizeMicro),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN12xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN16xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN24xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN24xlargeHighMemory),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN24xlargeOptimizedCpu),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN24xlargeOptimizedMemory),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN2xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN48xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN48xlargeHighMemory),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN48xlargeOptimizedCpu),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN48xlargeOptimizedMemory),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN4xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeN8xlarge),
			string(api.V1CreateProjectBodyDesiredInstanceSizeSmall),
			string(api.V1CreateProjectBodyDesiredInstanceSizeXlarge),
		},
	}

	// Original Supabase projects create flags kept for reference:
	// orgId string

	projectsCreateCmd = &cobra.Command{
		Use:     "create [project name]",
		Short:   "Create a Volcengine Supabase workspace",
		Args:    cobra.MaximumNArgs(1),
		Example: `byted-supabase-cli projects create my-project --region cn-beijing`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) || !interactive {
				cobra.CheckErr(cobra.MaximumNArgs(1)(cmd, args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				projectName = args[0]
			}
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			var suspendTimeoutPtr *int
			if cmd.Flags().Changed("suspend-timeout-seconds") {
				if suspendTimeout < -1 {
					return errors.New("--suspend-timeout-seconds must be -1 or greater")
				}
				suspendTimeoutPtr = &suspendTimeout
			}
			var isAgentPlanPtr *bool
			if cmd.Flags().Changed("is-agent-plan") {
				isAgentPlanPtr = &isAgentPlan
			}
			return create.RunVolcengine(cmd.Context(), create.RunVolcengineParams{
				WorkspaceName:         projectName,
				ProjectName:           volcProjectName,
				IsAgentPlan:           isAgentPlanPtr,
				AgentPlanSeatID:       agentPlanSeatID,
				Region:                volcengine.RegionSetting(),
				SuspendTimeoutSeconds: suspendTimeoutPtr,
			})

			// Original Supabase implementation:
			// body := api.V1CreateProjectBody{
			// 	Name:             projectName,
			// 	OrganizationSlug: orgId,
			// 	DbPass:           dbPassword,
			// 	Region:           cast.Ptr(api.V1CreateProjectBodyRegion(region.Value)),
			// }
			// if cmd.Flags().Changed("size") {
			// 	body.DesiredInstanceSize = (*api.V1CreateProjectBodyDesiredInstanceSize)(&size.Value)
			// }
			// return create.Run(cmd.Context(), body, afero.NewOsFs())
		},
	}

	projectsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all Supabase projects",
		Long:  "List all Supabase projects the logged-in user can access.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(projectsListLimit, projectsListOffset, false); err != nil {
				return err
			}
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			return list.Run(cmd.Context(), afero.NewOsFs(), list.RunParams{
				ProjectRef:  flags.ProjectRef,
				Detail:      projectDetail,
				ProjectName: volcProjectName,
				Limit:       projectsListLimit,
				Offset:      projectsListOffset,
			})
		},
	}

	projectsOverviewCmd = &cobra.Command{
		Use:   "overview",
		Short: "Show Volcengine Supabase workspace overview",
		Long:  "Show Volcengine Supabase workspace overview grouped by workspace status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			return overview.Run(cmd.Context(), overview.RunParams{
				ProjectName: volcProjectName,
			})
		},
	}

	projectsApiKeysCmd = &cobra.Command{
		Use:   "api-keys",
		Short: "List API keys for the default or specified branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, afero.NewOsFs(), "Which project do you want to list API keys for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
				return err
			}
			return apiKeys.RunVolcengine(ctx, apiKeys.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    volcBranchID,
				Fsys:        afero.NewOsFs(),
			})

			// Original Supabase implementation:
			// return apiKeys.Run(cmd.Context(), flags.ProjectRef, afero.NewOsFs())
		},
	}

	projectsOperationsCmd = &cobra.Command{
		Use:   "operations [ref/workspace-id]",
		Short: "List operation audit logs for a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
				return errors.New("missing workspace id. Supply projects operations [workspace-id].")
			}
			if err := validateVolcenginePageFlags(projectsOperationsLimit, projectsOperationsOffset, false); err != nil {
				return err
			}
			status, err := normalizeOperationStatus(projectsOperationsStatus)
			if err != nil {
				return err
			}
			projectsOperationsStatus = status
			return validateOperationTimeRange(projectsOperationsStartTime, projectsOperationsEndTime)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) > 0 {
				flags.ProjectRef = strings.TrimSpace(args[0])
				if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
					return err
				}
			} else {
				if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
					return err
				}
				if err := promptVolcengineProjectRef(ctx, "Which project do you want to list operation logs for?"); err != nil {
					return err
				}
			}
			return operationlist.Run(ctx, operationlist.Params{
				WorkspaceID:     flags.ProjectRef,
				BranchID:        strings.TrimSpace(projectsOperationsBranchID),
				ComputeID:       strings.TrimSpace(projectsOperationsComputeID),
				ActionName:      strings.TrimSpace(projectsOperationsActionName),
				Status:          projectsOperationsStatus,
				CreateTimeStart: strings.TrimSpace(projectsOperationsStartTime),
				CreateTimeEnd:   strings.TrimSpace(projectsOperationsEndTime),
				Limit:           projectsOperationsLimit,
				Offset:          projectsOperationsOffset,
			})
		},
	}

	projectsDeleteCmd = &cobra.Command{
		Use:   "delete [ref/workspace-id]",
		Short: "Delete a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
				return cobra.ExactArgs(1)(cmd, args)
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && !viper.GetBool("YES") {
				return errors.New("missing required flag: --yes")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if len(args) == 0 {
				if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
					return err
				}
				if err := promptVolcengineProjectRef(ctx, "Which project do you want to delete?"); err != nil {
					return err
				}
			} else {
				flags.ProjectRef = args[0]
				if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
					return err
				}
			}
			return delete.RunVolcengine(ctx, flags.ProjectRef, afero.NewOsFs())

			// Original Supabase implementation:
			// if err := delete.PreRun(ctx, flags.ProjectRef); err != nil {
			// 	return err
			// }
			// return delete.Run(ctx, flags.ProjectRef, afero.NewOsFs())
		},
	}

	projectsStartCmd = &cobra.Command{
		Use:   "start [ref/workspace-id]",
		Short: "Start a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
				return cobra.ExactArgs(1)(cmd, args)
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && !viper.GetBool("YES") {
				return errors.New("missing required flag: --yes")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to start?"); err != nil {
				return err
			}
			return delete.RunVolcengineStart(ctx, flags.ProjectRef, afero.NewOsFs())
		},
	}

	projectsStopCmd = &cobra.Command{
		Use:   "stop [ref/workspace-id]",
		Short: "Stop a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
				return cobra.ExactArgs(1)(cmd, args)
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && !viper.GetBool("YES") {
				return errors.New("missing required flag: --yes")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to stop?"); err != nil {
				return err
			}
			return delete.RunVolcengineStop(ctx, flags.ProjectRef, afero.NewOsFs())
		},
	}

	projectsRenameCmd = &cobra.Command{
		Use:   "rename [ref/workspace-id]",
		Short: "Rename a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireVolcengineMutationArgs(cmd, args); err != nil {
				return err
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && !cmd.Flags().Changed("name") {
				return errors.New("missing required flag: --name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to rename?"); err != nil {
				return err
			}
			if workspaceName == "" {
				name, err := utils.NewConsole().PromptText(ctx, "Enter new project name: ")
				if err != nil {
					return err
				}
				workspaceName = name
			}
			if workspaceName == "" {
				return errors.New("missing required flag: --name")
			}
			return manage.RenameWorkspace(ctx, manage.RenameParams{
				WorkspaceID:   flags.ProjectRef,
				WorkspaceName: workspaceName,
			})
		},
	}

	projectsDeletionProtectionCmd = &cobra.Command{
		Use:   "deletion-protection [ref/workspace-id]",
		Short: "Modify deletion protection for a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireVolcengineMutationArgs(cmd, args); err != nil {
				return err
			}
			enableChanged := cmd.Flags().Changed("enable")
			disableChanged := cmd.Flags().Changed("disable")
			if enableChanged && disableChanged {
				return errors.New("supply exactly one of --enable or --disable")
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && !enableChanged && !disableChanged {
				return errors.New("supply exactly one of --enable or --disable")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to modify deletion protection for?"); err != nil {
				return err
			}
			if !cmd.Flags().Changed("enable") && !cmd.Flags().Changed("disable") {
				enabled, err := promptVolcengineDeletionProtection(ctx)
				if err != nil {
					return err
				}
				enablePolicy = enabled
			}
			return manage.ModifyDeletionProtection(ctx, manage.DeletionProtectionParams{
				WorkspaceID: flags.ProjectRef,
				Enabled:     enablePolicy,
			})
		},
	}

	projectsCreateTagsCmd = &cobra.Command{
		Use:   "create-tags [ref/workspace-id]",
		Short: "Create tags for a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireVolcengineMutationArgs(cmd, args); err != nil {
				return err
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(tagValues) == 0 {
				return errors.New("missing required flag: --tag")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to create tags for?"); err != nil {
				return err
			}
			if len(tagValues) == 0 {
				values, err := promptVolcengineTags(ctx)
				if err != nil {
					return err
				}
				tagValues = values
			}
			tags, err := parseVolcengineTags(tagValues)
			if err != nil {
				return err
			}
			return manage.CreateTags(ctx, manage.TagsParams{
				WorkspaceID: flags.ProjectRef,
				Tags:        tags,
			})
		},
	}

	projectsDeleteTagsCmd = &cobra.Command{
		Use:   "delete-tags [ref/workspace-id]",
		Short: "Delete tags from a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireVolcengineMutationArgs(cmd, args); err != nil {
				return err
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(tagKeys) == 0 {
				return errors.New("missing required flag: --key")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to delete tags from?"); err != nil {
				return err
			}
			if len(tagKeys) == 0 {
				keys, err := promptVolcengineTagKeys(ctx, flags.ProjectRef)
				if err != nil {
					return err
				}
				tagKeys = keys
			}
			keys, err := parseVolcengineTagKeys(tagKeys)
			if err != nil {
				return err
			}
			return manage.DeleteTags(ctx, manage.TagsParams{
				WorkspaceID: flags.ProjectRef,
				TagKeys:     keys,
			})
		},
	}

	projectsComputeSettingsCmd = &cobra.Command{
		Use:   "compute-settings [ref/workspace-id]",
		Short: "Modify compute settings for a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireVolcengineMutationArgs(cmd, args); err != nil {
				return err
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				if !cmd.Flags().Changed("min-cu") {
					return errors.New("missing required flag: --min-cu")
				}
				if !cmd.Flags().Changed("max-cu") {
					return errors.New("missing required flag: --max-cu")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serviceType, err := normalizeComputeServiceType(computeSettingsServiceType)
			if err != nil {
				return err
			}
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to modify compute settings for?"); err != nil {
				return err
			}
			current, err := describeVolcengineComputeSettingsForPrompt(ctx, flags.ProjectRef, serviceType)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("min-cu") {
				value, err := promptVolcengineFloat64(ctx, fmt.Sprintf("Enter %s AutoScalingLimitMinCU", serviceType), current.AutoScalingLimitMinCU)
				if err != nil {
					return err
				}
				minCU = value
			}
			if !cmd.Flags().Changed("max-cu") {
				value, err := promptVolcengineFloat64(ctx, fmt.Sprintf("Enter %s AutoScalingLimitMaxCU", serviceType), current.AutoScalingLimitMaxCU)
				if err != nil {
					return err
				}
				maxCU = value
			}
			var suspendTimeoutPtr *int
			if cmd.Flags().Changed("suspend-timeout-seconds") {
				suspendTimeoutPtr = &suspendTimeout
			}
			if err := validateVolcengineComputeSettingsValues(serviceType, minCU, maxCU, suspendTimeoutPtr); err != nil {
				return err
			}
			return manage.ModifyComputeSettings(ctx, manage.ComputeSettingsParams{
				WorkspaceID:           flags.ProjectRef,
				AutoScalingLimitMinCU: minCU,
				AutoScalingLimitMaxCU: maxCU,
				SuspendTimeoutSeconds: suspendTimeoutPtr,
				ServiceType:           serviceType,
			})
		},
	}

	projectsWorkspaceSettingsCmd = &cobra.Command{
		Use:   "workspace-settings [ref/workspace-id]",
		Short: "Modify workspace settings for a Volcengine Supabase workspace",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireVolcengineMutationArgs(cmd, args); err != nil {
				return err
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) && !cmd.Flags().Changed("history-retention-hours") {
				return errors.New("missing required flag: --history-retention-hours")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := loadVolcengineMutationProjectRef(ctx, args, "Which project do you want to modify workspace settings for?"); err != nil {
				return err
			}
			if !cmd.Flags().Changed("history-retention-hours") {
				current, err := describeVolcengineWorkspaceForPrompt(ctx, flags.ProjectRef)
				if err != nil {
					return err
				}
				value, err := promptVolcengineInt(ctx, "Enter HistoryRetentionHours", current.WorkspaceSetting.HistoryRetentionHours)
				if err != nil {
					return err
				}
				retentionHours = value
			}
			return manage.ModifyWorkspaceSettings(ctx, manage.WorkspaceSettingsParams{
				WorkspaceID:           flags.ProjectRef,
				HistoryRetentionHours: retentionHours,
			})
		},
	}
)

func init() {
	// Add flags to cobra command
	createFlags := projectsCreateCmd.Flags()
	createFlags.BoolVarP(&interactive, "interactive", "i", true, "Enables interactive mode.")
	cobra.CheckErr(createFlags.MarkHidden("interactive"))
	createFlags.StringVar(&volcProjectName, "volc-project-name", "", "Volcengine ProjectName for the new workspace.")
	createFlags.BoolVar(&isAgentPlan, "is-agent-plan", false, "Create a personal Agent Plan workspace. When Agent Plan flags are omitted, use the current profile default.")
	createFlags.StringVar(&agentPlanSeatID, "agent-plan-seat-id", "", "Create an enterprise Agent Plan workspace with this seat ID. When Agent Plan flags are omitted, use the current profile default.")
	createFlags.IntVar(&suspendTimeout, "suspend-timeout-seconds", 0, "Auto-suspend idle timeout in seconds for the new workspace: -1 disables auto-suspend (always-on), or 300-604800 to enable (e.g. 3600 = 60 min). When omitted for a small Agent Plan workspace (Small/Medium, personal or enterprise edition), defaults to 3600 (60 min).")

	// Original Supabase create flags kept for reference:
	// createFlags.StringVar(&orgId, "org-id", "", "Organization ID to create the project in.")
	// markFlagTelemetrySafe(createFlags.Lookup("org-id"))
	// createFlags.StringVar(&dbPassword, "db-password", "", "Database password of the project.")
	// createFlags.Var(&region, "region", "Select a region close to you for the best performance.")
	// createFlags.String("plan", "", "Select a plan that suits your needs.")
	// cobra.CheckErr(createFlags.MarkHidden("plan"))
	// createFlags.Var(&size, "size", "Select a desired instance size for your project.")
	// cobra.CheckErr(viper.BindPFlag("DB_PASSWORD", createFlags.Lookup("db-password")))

	listFlags := projectsListCmd.Flags()
	listFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id for Volcengine.")
	markFlagTelemetrySafe(listFlags.Lookup("project-ref"))
	listFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	markFlagTelemetrySafe(listFlags.Lookup("workspace-id"))
	listFlags.BoolVar(&projectDetail, "detail", false, "Show Volcengine workspace detail instead of listing all projects.")
	listFlags.StringVar(&volcProjectName, "volc-project-name", "", "Volcengine ProjectName to filter workspaces.")
	listFlags.IntVar(&projectsListLimit, "limit", volcengineDefaultListLimit, "Maximum number of workspaces to return (1-100).")
	listFlags.IntVar(&projectsListOffset, "offset", 0, "Number of workspaces to skip before returning results.")

	overviewFlags := projectsOverviewCmd.Flags()
	overviewFlags.StringVar(&volcProjectName, "volc-project-name", "", "Volcengine ProjectName to filter workspace overview.")

	apiKeysFlags := projectsApiKeysCmd.Flags()
	apiKeysFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(apiKeysFlags.Lookup("project-ref"))
	apiKeysFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(apiKeysFlags.Lookup("workspace-id"))
	apiKeysFlags.StringVar(&volcBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase workspace.")

	operationsFlags := projectsOperationsCmd.Flags()
	operationsFlags.StringVar(&projectsOperationsBranchID, "branch-id", "", "Branch ID to filter operations.")
	operationsFlags.StringVar(&projectsOperationsComputeID, "compute-id", "", "Compute ID to filter operations.")
	operationsFlags.StringVar(&projectsOperationsActionName, "action-name", "", "Action name to filter operations.")
	operationsFlags.StringVar(&projectsOperationsStatus, "status", "", "Operation status filter: Start, Running, Success, or Failed.")
	operationsFlags.StringVar(&projectsOperationsStartTime, "start-time", "", "Filter operations created at or after this RFC3339 UTC timestamp.")
	operationsFlags.StringVar(&projectsOperationsEndTime, "end-time", "", "Filter operations created at or before this RFC3339 UTC timestamp.")
	operationsFlags.IntVar(&projectsOperationsLimit, "limit", 10, "Maximum number of operations to return (1-100).")
	operationsFlags.IntVar(&projectsOperationsOffset, "offset", 0, "Number of operations to skip before returning results.")

	renameFlags := projectsRenameCmd.Flags()
	renameFlags.StringVar(&workspaceName, "name", "", "New Volcengine Supabase workspace name.")

	deletionProtectionFlags := projectsDeletionProtectionCmd.Flags()
	deletionProtectionFlags.BoolVar(&enablePolicy, "enable", false, "Enable deletion protection.")
	deletionProtectionFlags.BoolVar(&disablePolicy, "disable", false, "Disable deletion protection.")

	createTagsFlags := projectsCreateTagsCmd.Flags()
	createTagsFlags.StringArrayVar(&tagValues, "tag", []string{}, "Tag to create in key=value format. Can be specified multiple times.")

	deleteTagsFlags := projectsDeleteTagsCmd.Flags()
	deleteTagsFlags.StringSliceVar(&tagKeys, "key", []string{}, "Tag key to delete.")

	computeSettingsFlags := projectsComputeSettingsCmd.Flags()
	computeSettingsFlags.Float64Var(&minCU, "min-cu", 0, "Minimum compute CU for autoscaling.")
	computeSettingsFlags.Float64Var(&maxCU, "max-cu", 0, "Maximum compute CU for autoscaling.")
	computeSettingsFlags.IntVar(&suspendTimeout, "suspend-timeout-seconds", 0, "Auto-suspend idle timeout in seconds: -1 disables auto-suspend (always-on), or 300-604800 to enable (e.g. 3600 = 60 min).")
	computeSettingsFlags.StringVar(&computeSettingsServiceType, "service-type", volcengine.ServiceTypeSupabase, "Compute service type to modify: Supabase or Database.")

	workspaceSettingsFlags := projectsWorkspaceSettingsCmd.Flags()
	workspaceSettingsFlags.IntVar(&retentionHours, "history-retention-hours", 0, "History retention hours.")

	// Add commands to root
	projectsCmd.AddCommand(projectsCreateCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsOverviewCmd)
	projectsCmd.AddCommand(projectsApiKeysCmd)
	projectsCmd.AddCommand(projectsOperationsCmd)
	projectsCmd.AddCommand(projectsStartCmd)
	projectsCmd.AddCommand(projectsStopCmd)
	projectsCmd.AddCommand(projectsRenameCmd)
	projectsCmd.AddCommand(projectsDeletionProtectionCmd)
	projectsCmd.AddCommand(projectsCreateTagsCmd)
	projectsCmd.AddCommand(projectsDeleteTagsCmd)
	projectsCmd.AddCommand(projectsComputeSettingsCmd)
	projectsCmd.AddCommand(projectsWorkspaceSettingsCmd)
	rootCmd.AddCommand(projectsCmd)
}

func validateVolcenginePageFlags(limit, offset int, allowZeroLimit bool) error {
	if limit < 0 || (!allowZeroLimit && limit == 0) || limit > 100 {
		if allowZeroLimit {
			return errors.New("invalid --limit. Expected 0 to list all items, or a value from 1 to 100.")
		}
		return errors.New("invalid --limit. Expected a value from 1 to 100.")
	}
	if offset < 0 {
		return errors.New("invalid --offset. Expected a value greater than or equal to 0.")
	}
	return nil
}

func normalizeOperationStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "start":
		return "Start", nil
	case "running":
		return "Running", nil
	case "success":
		return "Success", nil
	case "failed":
		return "Failed", nil
	default:
		return "", errors.New("invalid --status. Expected Start, Running, Success, or Failed.")
	}
}

func validateOperationTimeRange(start, end string) error {
	var startTime, endTime time.Time
	var err error
	if strings.TrimSpace(start) != "" {
		startTime, err = time.Parse(time.RFC3339, strings.TrimSpace(start))
		if err != nil {
			return errors.New("invalid --start-time. Expected RFC3339 format like 2026-05-25T10:00:00Z.")
		}
	}
	if strings.TrimSpace(end) != "" {
		endTime, err = time.Parse(time.RFC3339, strings.TrimSpace(end))
		if err != nil {
			return errors.New("invalid --end-time. Expected RFC3339 format like 2026-05-25T10:00:00Z.")
		}
	}
	if !startTime.IsZero() && !endTime.IsZero() && startTime.After(endTime) {
		return errors.New("--start-time must be earlier than or equal to --end-time.")
	}
	return nil
}

func requireVolcengineMutationArgs(cmd *cobra.Command, args []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
		return cobra.ExactArgs(1)(cmd, args)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) && !viper.GetBool("YES") {
		return errors.New("missing required flag: --yes")
	}
	return nil
}

func loadVolcengineMutationProjectRef(ctx context.Context, args []string, prompt string) error {
	if len(args) == 0 {
		if err := ensureVolcengineRegion(ctx, afero.NewOsFs(), false); err != nil {
			return err
		}
		return promptVolcengineProjectRef(ctx, prompt)
	}
	flags.ProjectRef = args[0]
	return ensureVolcengineRegion(ctx, afero.NewOsFs(), false)
}

func promptVolcengineDeletionProtection(ctx context.Context) (bool, error) {
	items := []utils.PromptItem{
		{
			Summary: "enable",
			Details: "Enable deletion protection.",
		},
		{
			Summary: "disable",
			Details: "Disable deletion protection.",
		},
	}
	choice, err := utils.PromptChoice(ctx, "Select deletion protection policy:", items)
	if err != nil {
		return false, err
	}
	return choice.Summary == "enable", nil
}

func promptVolcengineTags(ctx context.Context) ([]string, error) {
	input, err := utils.NewConsole().PromptText(ctx, "Enter tags in key=value format, separated by spaces: ")
	if err != nil {
		return nil, err
	}
	values, err := splitVolcengineTagFields(input)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errors.New("missing required flag: --tag")
	}
	return values, nil
}

func promptVolcengineTagKeys(ctx context.Context, workspaceID string) ([]string, error) {
	workspace, err := describeVolcengineWorkspaceForPrompt(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(workspace.WorkspaceTags) == 0 {
		return nil, errors.Errorf("workspace %s has no tags to delete", workspaceID)
	}
	items := make([]utils.PromptItem, 0, len(workspace.WorkspaceTags))
	for _, tag := range workspace.WorkspaceTags {
		if tag.Key == "" {
			continue
		}
		details := "value: " + tag.Value
		if tag.System {
			details += ", system: true"
		}
		items = append(items, utils.PromptItem{
			Summary: tag.Key,
			Details: details,
		})
	}
	if len(items) == 0 {
		return nil, errors.Errorf("workspace %s has no tags to delete", workspaceID)
	}
	choice, err := utils.PromptChoice(ctx, "Which tag key do you want to delete?", items)
	if err != nil {
		return nil, err
	}
	return []string{choice.Summary}, nil
}

func promptVolcengineFloat64(ctx context.Context, label string, current float64) (float64, error) {
	input, err := utils.NewConsole().PromptText(ctx, fmt.Sprintf("%s (current: %v): ", label, current))
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(input) == "" {
		return current, nil
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
	if err != nil {
		return 0, errors.Errorf("invalid %s: %w", label, err)
	}
	return value, nil
}

func promptVolcengineInt(ctx context.Context, label string, current int) (int, error) {
	input, err := utils.NewConsole().PromptText(ctx, fmt.Sprintf("%s (current: %d): ", label, current))
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(input) == "" {
		return current, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, errors.Errorf("invalid %s: %w", label, err)
	}
	return value, nil
}

func describeVolcengineComputeSettingsForPrompt(ctx context.Context, workspaceID, serviceType string) (volcengine.Compute, error) {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return volcengine.Compute{}, err
	}
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		client := volcengine.NewClient(cfg)
		detail, err := client.DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := detail.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Compute{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		defaultBranch, err := client.DescribeDefaultBranch(ctx, workspace.WorkspaceID)
		if err != nil {
			return volcengine.Compute{}, err
		}
		branchID := defaultBranch.Branch.BranchID
		if branchID == "" {
			return volcengine.Compute{}, errors.Errorf("failed to resolve default branch for workspace %s", workspace.WorkspaceID)
		}
		computes, err := client.DescribeComputes(ctx, workspace.WorkspaceID, branchID, serviceType)
		if err != nil {
			return volcengine.Compute{}, err
		}
		for _, compute := range computes.Computes {
			if strings.EqualFold(compute.ServiceType, serviceType) {
				return compute, nil
			}
		}
		return volcengine.Compute{}, errors.Errorf("no %s compute found for workspace %s branch %s", serviceType, workspace.WorkspaceID, branchID)
	}
	if lastErr != nil {
		return volcengine.Compute{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Compute{}, errors.Errorf("failed to describe volcengine project %s", workspaceID)
}

func validateVolcengineComputeSettingsValues(serviceType string, minCU, maxCU float64, suspendTimeout *int) error {
	if !isValidVolcengineComputeCU(minCU) {
		return errors.New("invalid --min-cu. Expected 0.25, 0.5, or an integer between 1 and 32.")
	}
	if !isValidVolcengineComputeCU(maxCU) {
		return errors.New("invalid --max-cu. Expected 0.25, 0.5, or an integer between 1 and 32.")
	}
	if serviceType == volcengine.ServiceTypeSupabase && minCU < 0.5 {
		return errors.New("invalid --min-cu for Supabase service type. Expected 0.5 or greater.")
	}
	if minCU < 0.25 {
		return errors.New("invalid --min-cu. Expected 0.25 or greater.")
	}
	if maxCU > 32 {
		return errors.New("invalid --max-cu. Expected 32 or less.")
	}
	if maxCU < minCU {
		return errors.New("invalid --max-cu. Expected --max-cu to be greater than or equal to --min-cu.")
	}
	if maxCU > minCU*8 {
		return errors.New("invalid compute CU range. Expected --max-cu to be at most 8 times --min-cu.")
	}
	if suspendTimeout != nil {
		if *suspendTimeout < -1 || (*suspendTimeout > 0 && *suspendTimeout < 300) || *suspendTimeout > 604800 {
			return errors.New("invalid --suspend-timeout-seconds. Expected -1, 0, or a value between 300 and 604800.")
		}
	}
	return nil
}

func isValidVolcengineComputeCU(value float64) bool {
	if value == 0.25 || value == 0.5 {
		return true
	}
	if value < 1 || value > 32 {
		return false
	}
	return value == float64(int(value))
}

func describeVolcengineWorkspaceForPrompt(ctx context.Context, workspaceID string) (volcengine.Workspace, error) {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return volcengine.Workspace{}, err
	}
	var lastErr error
	for _, region := range cfgSet.Regions {
		result, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := result.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Workspace{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		return workspace, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, errors.Errorf("volcengine project %s not found", workspaceID)
}

func parseVolcengineTags(values []string) ([]volcengine.WorkspaceTag, error) {
	pairs, err := expandVolcengineTagValues(values)
	if err != nil {
		return nil, err
	}
	tags := make([]volcengine.WorkspaceTag, 0, len(pairs))
	for _, value := range pairs {
		key, tagValue, ok := parseVolcengineTagPair(value)
		if !ok || key == "" {
			return nil, errors.Errorf("invalid tag %q. Use key=value format.", value)
		}
		tags = append(tags, volcengine.WorkspaceTag{
			Key:   key,
			Value: tagValue,
		})
	}
	return tags, nil
}

func expandVolcengineTagValues(values []string) ([]string, error) {
	var pairs []string
	for _, value := range values {
		fields, err := splitVolcengineTagFields(value)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, fields...)
	}
	return pairs, nil
}

func splitVolcengineTagFields(input string) ([]string, error) {
	var fields []string
	var builder strings.Builder
	var quote rune
	escaped := false
	for _, r := range input {
		if escaped {
			builder.WriteRune('\\')
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			builder.WriteRune(r)
			if r == quote {
				quote = 0
			}
			continue
		}
		switch {
		case r == '"':
			quote = r
			builder.WriteRune(r)
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if field := strings.TrimSpace(builder.String()); field != "" {
				fields = append(fields, field)
				builder.Reset()
			}
		default:
			builder.WriteRune(r)
		}
	}
	if escaped {
		builder.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote in tag value")
	}
	if field := strings.TrimSpace(builder.String()); field != "" {
		fields = append(fields, field)
	}
	return fields, nil
}

func parseVolcengineTagPair(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, '\'') {
		return "", "", false
	}
	separator := findVolcengineTagSeparator(value)
	if separator < 0 {
		return "", "", false
	}
	key, ok := unquoteVolcengineTagPart(value[:separator])
	if !ok {
		return "", "", false
	}
	tagValue, ok := unquoteVolcengineTagPart(value[separator+1:])
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(tagValue), true
}

func findVolcengineTagSeparator(value string) int {
	var quote rune
	escaped := false
	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' {
			quote = r
			continue
		}
		if r == '=' {
			return i
		}
	}
	return -1
}

func unquoteVolcengineTagPart(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		quote := value[0]
		if quote == '"' && value[len(value)-1] == quote {
			return unescapeVolcengineTagPart(value[1 : len(value)-1]), true
		}
		if quote == '"' || value[len(value)-1] == '"' {
			return "", false
		}
	}
	return unescapeVolcengineTagPart(value), true
}

func unescapeVolcengineTagPart(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	escaped := false
	for _, r := range value {
		if escaped {
			builder.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		builder.WriteRune(r)
	}
	if escaped {
		builder.WriteRune('\\')
	}
	return builder.String()
}

func parseVolcengineTagKeys(values []string) ([]string, error) {
	keys := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			return nil, errors.New("tag key cannot be empty")
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func promptVolcengineProjectRef(ctx context.Context, title string) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	var workspaces []volcengine.Workspace
	for _, region := range cfgSet.Regions {
		result, err := volcengine.NewClient(cfgSet.ConfigForRegion(region)).ListWorkspaces(ctx, volcengine.ListWorkspacesParams{
			Limit: volcenginePromptWorkspaceLimit,
		})
		if err != nil {
			return errors.Errorf("failed to list volcengine projects in region %s: %w", region, err)
		}
		if result.Total > len(result.Workspaces) {
			return errors.Errorf(
				"too many Supabase projects found in region %s (%d). Supply --workspace-id/--project-ref directly.",
				region,
				result.Total,
			)
		}
		workspaces = append(workspaces, result.Workspaces...)
	}
	if len(workspaces) == 0 {
		return errors.New("no volcengine Supabase projects found")
	}
	items := make([]utils.PromptItem, len(workspaces))
	for i, workspace := range workspaces {
		items[i] = utils.PromptItem{
			Summary: workspace.WorkspaceID,
			Details: "name: " + workspace.WorkspaceName + ", region: " + workspace.RegionID,
		}
	}
	choice, err := utils.PromptChoice(ctx, title, items)
	if err != nil {
		return err
	}
	flags.ProjectRef = choice.Summary
	fmt.Fprintln(os.Stderr, "Selected project:", flags.ProjectRef)
	return nil
}

func loadOrPromptVolcengineProjectRef(ctx context.Context, fsys afero.Fs, titles ...string) error {
	if flags.ProjectRef != "" {
		if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
			return err
		}
		return nil
	}
	linkedWorkspaceID, err := volcengine.LoadLinkedWorkspaceID(fsys)
	if err != nil {
		return err
	}
	if linkedWorkspaceID != "" {
		flags.ProjectRef = linkedWorkspaceID
		if err := ensureVolcengineRegion(ctx, fsys, true); err != nil {
			return err
		}
		return nil
	}
	if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
		return err
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		title := "Select a project:"
		if len(titles) > 0 && titles[0] != "" {
			title = titles[0]
		}
		return promptVolcengineProjectRef(ctx, title)
	}
	return errors.New("missing workspace id. Supply --workspace-id, --project-ref, or run byted-supabase-cli link first.")
}

func ensureVolcengineRegion(ctx context.Context, fsys afero.Fs, allowLinked bool) error {
	if volcengine.HasRegionOverride() {
		return nil
	}
	if allowLinked {
		linkedRegion, err := volcengine.LoadLinkedRegion(fsys)
		if err != nil {
			return err
		}
		if linkedRegion != "" {
			volcengine.SetRegionOverride(linkedRegion)
			return nil
		}
	}
	if volcengine.RegionSetting() != "" {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		volcengine.SetRegionOverride(volcengine.DefaultRegion)
		fmt.Fprintln(os.Stderr, "No region configured; using default region:", volcengine.DefaultRegion)
		return nil
	}
	region, err := promptVolcengineRegion(ctx)
	if err != nil {
		return err
	}
	volcengine.SetRegionOverride(region)
	fmt.Fprintln(os.Stderr, "Selected region:", region)
	return nil
}

func promptVolcengineRegion(ctx context.Context) (string, error) {
	regions := volcengine.ConfiguredRegions()
	if len(regions) == 0 {
		return volcengine.DefaultRegion, nil
	}
	items := make([]utils.PromptItem, len(regions))
	for i, region := range regions {
		items[i] = utils.PromptItem{
			Summary: region,
			Details: region,
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a Volcengine region:", items)
	if err != nil {
		return "", err
	}
	return choice.Summary, nil
}
