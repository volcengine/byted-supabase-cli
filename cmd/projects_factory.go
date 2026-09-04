// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	operationlist "github.com/volcengine/byted-supabase-cli/internal/operations/list"
	"github.com/volcengine/byted-supabase-cli/internal/projects/apiKeys"
	"github.com/volcengine/byted-supabase-cli/internal/projects/create"
	projectdelete "github.com/volcengine/byted-supabase-cli/internal/projects/delete"
	projectlist "github.com/volcengine/byted-supabase-cli/internal/projects/list"
	"github.com/volcengine/byted-supabase-cli/internal/projects/manage"
	"github.com/volcengine/byted-supabase-cli/internal/projects/overview"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

type projectsFactoryOptions struct {
	WorkspaceID string
}

func projectsCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newProjectsCommand}}
}

func newProjectsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{GroupID: groupManagementAPI, Use: "projects", Short: "Manage Volcengine Supabase Workspaces"}
	cmd.AddCommand(
		newProjectsCreateCommand(ctx),
		newProjectsDeleteCommand(ctx),
		newProjectsListCommand(ctx),
		newProjectsOverviewCommand(ctx),
		newProjectsAPIKeysCommand(ctx),
		newProjectsOperationsCommand(ctx),
		newProjectsStartCommand(ctx),
		newProjectsStopCommand(ctx),
		newProjectsSuspendCommand(ctx),
		newProjectsWakeCommand(ctx),
		newProjectsSettingsCommand(ctx),
		newProjectsCreateTagsCommand(ctx),
		newProjectsDeleteTagsCommand(ctx),
	)
	return cmd
}

func newProjectsSettingsComputeCommand(ctx RuntimeContext) *cobra.Command {
	var min, max float64
	var suspend int
	var serviceType string
	options := &projectsFactoryOptions{}
	cmd := &cobra.Command{
		Use:   "compute",
		Short: "Configure workspace compute and auto-sleep settings",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				if strings.TrimSpace(options.WorkspaceID) == "" {
					return errors.New("missing workspace id. Supply --workspace-id, --project-ref, or run byted-supabase-cli link first.")
				}
				if !viper.GetBool("YES") {
					return errors.New("missing required flag: --yes")
				}
				if !cmd.Flags().Changed("min-cu") || !cmd.Flags().Changed("max-cu") {
					return errors.New("missing required flag: --min-cu and --max-cu")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveSettingsWorkspace(cmd, options, "Which project do you want to modify workspace compute settings for?"); err != nil {
				return err
			}
			serviceType, err := normalizeComputeServiceType(serviceType)
			if err != nil {
				return err
			}
			current, err := describeVolcengineComputeSettingsForPrompt(cmd.Context(), options.WorkspaceID, serviceType)
			if err != nil {
				return err
			}
			if !cmd.Flags().Changed("min-cu") {
				min, err = promptVolcengineFloat64(cmd.Context(), fmt.Sprintf("Enter %s AutoScalingLimitMinCU", serviceType), current.AutoScalingLimitMinCU)
				if err != nil {
					return err
				}
			}
			if !cmd.Flags().Changed("max-cu") {
				max, err = promptVolcengineFloat64(cmd.Context(), fmt.Sprintf("Enter %s AutoScalingLimitMaxCU", serviceType), current.AutoScalingLimitMaxCU)
				if err != nil {
					return err
				}
			}
			var timeout *int
			if cmd.Flags().Changed("suspend-timeout-seconds") {
				timeout = &suspend
			}
			if err := validateVolcengineComputeSettingsValues(serviceType, min, max, timeout); err != nil {
				return err
			}
			return manage.ModifyComputeSettings(cmd.Context(), manage.ComputeSettingsParams{
				WorkspaceID: options.WorkspaceID, AutoScalingLimitMinCU: min, AutoScalingLimitMaxCU: max,
				SuspendTimeoutSeconds: timeout, ServiceType: serviceType,
			})
		},
	}
	cmd.Flags().Float64Var(&min, "min-cu", 0, "Minimum compute CU for workspace service autoscaling.")
	cmd.Flags().Float64Var(&max, "max-cu", 0, "Maximum compute CU for workspace service autoscaling.")
	cmd.Flags().IntVar(&suspend, "suspend-timeout-seconds", 0, "Workspace service compute auto-sleep idle timeout: -1 disables auto-sleep, or 300-604800 seconds to enable.")
	cmd.Flags().StringVar(&serviceType, "service-type", volcengine.ServiceTypeSupabase, "Workspace service type: Supabase or Database.")
	newProjectsWorkspaceFlags(cmd, options)
	_ = ctx
	return cmd
}

func newProjectsSettingsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Configure workspace settings",
	}
	cmd.AddCommand(
		newProjectsSettingsComputeCommand(ctx),
		newProjectsSettingsNameCommand(ctx),
		newProjectsSettingsRetentionCommand(ctx),
		newProjectsSettingsDeletionProtectionCommand(ctx),
		newProjectsSettingsNetworkCommand(ctx),
		newProjectsSettingsPrivateDNSCommand(ctx),
	)
	return cmd
}

func newProjectsWorkspaceFlags(cmd *cobra.Command, options *projectsFactoryOptions) {
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("project-ref"))
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("workspace-id"))
}

func resolveProjectsWorkspace(cmd *cobra.Command, options *projectsFactoryOptions, prompt string) error {
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

func resolveProjectsMutation(cmd *cobra.Command, args []string, prompt string) (string, error) {
	previous := flags.ProjectRef
	defer func() { flags.ProjectRef = previous }()
	if len(args) > 0 {
		flags.ProjectRef = strings.TrimSpace(args[0])
	} else {
		if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
			return "", err
		}
		if err := promptVolcengineProjectRef(cmd.Context(), prompt); err != nil {
			return "", err
		}
	}
	workspaceID := flags.ProjectRef
	if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func requireProjectsMutation(cmd *cobra.Command, args []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
		return cobra.ExactArgs(1)(cmd, args)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) && !viper.GetBool("YES") {
		return errors.New("missing required flag: --yes")
	}
	return nil
}

func newProjectsCreateCommand(ctx RuntimeContext) *cobra.Command {
	var options struct {
		Interactive    bool
		ProjectName    string
		WorkspaceName  string
		AgentPlan      bool
		SeatID         string
		SuspendTimeout int
	}
	cmd := &cobra.Command{
		Use: "create [project name]", Short: "Create a Volcengine Supabase workspace",
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) || !options.Interactive {
				return cobra.MaximumNArgs(1)(cmd, args)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				options.WorkspaceName = args[0]
			}
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			var timeout *int
			if cmd.Flags().Changed("suspend-timeout-seconds") {
				if options.SuspendTimeout < -1 {
					return errors.New("--suspend-timeout-seconds must be -1 or greater")
				}
				timeout = &options.SuspendTimeout
			}
			var agentPlan *bool
			if cmd.Flags().Changed("is-agent-plan") {
				agentPlan = &options.AgentPlan
			}
			return create.RunVolcengine(cmd.Context(), create.RunVolcengineParams{
				WorkspaceName: options.WorkspaceName, ProjectName: options.ProjectName,
				IsAgentPlan: agentPlan, AgentPlanSeatID: options.SeatID,
				Region: volcengine.RegionSetting(), SuspendTimeoutSeconds: timeout,
			})
		},
	}
	cmd.Flags().BoolVarP(&options.Interactive, "interactive", "i", true, "Enables interactive mode.")
	_ = cmd.Flags().MarkHidden("interactive")
	cmd.Flags().StringVar(&options.ProjectName, "volc-project-name", "", "Volcengine ProjectName for the new workspace.")
	cmd.Flags().BoolVar(&options.AgentPlan, "is-agent-plan", false, "Create a personal Agent Plan workspace. When Agent Plan flags are omitted, use the current profile default.")
	cmd.Flags().StringVar(&options.SeatID, "agent-plan-seat-id", "", "Create an enterprise Agent Plan workspace with this seat ID. When Agent Plan flags are omitted, use the current profile default.")
	cmd.Flags().IntVar(&options.SuspendTimeout, "suspend-timeout-seconds", 0, "Auto-suspend idle timeout in seconds.")
	_ = ctx
	return cmd
}

func newProjectsListCommand(ctx RuntimeContext) *cobra.Command {
	var options projectsFactoryOptions
	var detail bool
	var name string
	var limit, offset int
	cmd := &cobra.Command{Use: "list", Short: "List all Supabase projects", Long: "List all Supabase projects the logged-in user can access.", RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateVolcenginePageFlags(limit, offset, true); err != nil {
			return err
		}
		if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
			return err
		}
		return projectlist.Run(cmd.Context(), afero.NewOsFs(), projectlist.RunParams{ProjectRef: options.WorkspaceID, Detail: detail, ProjectName: name, Limit: limit, Offset: offset})
	}}
	newProjectsWorkspaceFlags(cmd, &options)
	cmd.Flags().BoolVar(&detail, "detail", false, "Show Volcengine workspace detail instead of listing all projects.")
	cmd.Flags().StringVar(&name, "volc-project-name", "", "Volcengine ProjectName to filter workspaces.")
	cmd.Flags().IntVar(&limit, "limit", volcengineDefaultListLimit, "Maximum number of workspaces to return (1-100). Set 0 to list all.")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of workspaces to skip before returning results.")
	_ = ctx
	return cmd
}

func newProjectsOverviewCommand(ctx RuntimeContext) *cobra.Command {
	var name string
	cmd := &cobra.Command{Use: "overview", Short: "Show Volcengine Supabase workspace overview", RunE: func(cmd *cobra.Command, args []string) error {
		if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
			return err
		}
		return overview.Run(cmd.Context(), overview.RunParams{ProjectName: name})
	}}
	cmd.Flags().StringVar(&name, "volc-project-name", "", "Volcengine ProjectName to filter workspace overview.")
	_ = ctx
	return cmd
}

func newProjectsAPIKeysCommand(ctx RuntimeContext) *cobra.Command {
	var options projectsFactoryOptions
	var projectName, branchID string
	cmd := &cobra.Command{Use: "api-keys", Short: "List API keys for the default or specified branch", RunE: func(cmd *cobra.Command, args []string) error {
		if err := resolveProjectsWorkspace(cmd, &options, "Which project do you want to list API keys for?"); err != nil {
			return err
		}
		return apiKeys.RunVolcengine(cmd.Context(), apiKeys.RunVolcengineParams{ProjectName: projectName, WorkspaceID: options.WorkspaceID, BranchID: branchID, Fsys: afero.NewOsFs()})
	}}
	newProjectsWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&projectName, "project-name", "", "Volcengine ProjectName sent to the AI Gateway API.")
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID of the Volcengine Supabase workspace.")
	_ = ctx
	return cmd
}

func newProjectsOperationsCommand(ctx RuntimeContext) *cobra.Command {
	var branchID, computeID, actionName, status, start, end string
	var limit, offset int
	var workspaceID string
	cmd := &cobra.Command{Use: "operations [ref/workspace-id]", Short: "List operation audit logs for a Volcengine Supabase workspace", Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) && len(args) == 0 {
				return errors.New("missing workspace id. Supply projects operations [workspace-id].")
			}
			if err := validateVolcenginePageFlags(limit, offset, false); err != nil {
				return err
			}
			normalized, err := normalizeOperationStatus(status)
			if err != nil {
				return err
			}
			status = normalized
			return validateOperationTimeRange(start, end)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				workspaceID = strings.TrimSpace(args[0])
			}
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			if workspaceID == "" {
				if err := promptVolcengineProjectRef(cmd.Context(), "Which project do you want to list operation logs for?"); err != nil {
					return err
				}
				workspaceID = flags.ProjectRef
			}
			return operationlist.Run(cmd.Context(), operationlist.Params{WorkspaceID: workspaceID, BranchID: strings.TrimSpace(branchID), ComputeID: strings.TrimSpace(computeID), ActionName: strings.TrimSpace(actionName), Status: status, CreateTimeStart: strings.TrimSpace(start), CreateTimeEnd: strings.TrimSpace(end), Limit: limit, Offset: offset})
		}}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID to filter operations.")
	cmd.Flags().StringVar(&computeID, "compute-id", "", "Compute ID to filter operations.")
	cmd.Flags().StringVar(&actionName, "action-name", "", "Action name to filter operations.")
	cmd.Flags().StringVar(&status, "status", "", "Operation status filter: Start, Running, Success, or Failed.")
	cmd.Flags().StringVar(&start, "start-time", "", "Filter operations created at or after this RFC3339 UTC timestamp.")
	cmd.Flags().StringVar(&end, "end-time", "", "Filter operations created at or before this RFC3339 UTC timestamp.")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of operations to return (1-100).")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of operations to skip.")
	_ = ctx
	return cmd
}

func newProjectsDeleteCommand(ctx RuntimeContext) *cobra.Command {
	return newProjectsLifecycleCommand(ctx, "delete", "Delete a Volcengine Supabase workspace", func(c *cobra.Command, id string) error {
		return projectdelete.RunVolcengine(c.Context(), id, afero.NewOsFs())
	})
}
func newProjectsStartCommand(ctx RuntimeContext) *cobra.Command {
	return newProjectsLifecycleCommand(ctx, "start", "Start a Volcengine Supabase workspace", func(c *cobra.Command, id string) error {
		return projectdelete.RunVolcengineStart(c.Context(), id, afero.NewOsFs())
	})
}
func newProjectsStopCommand(ctx RuntimeContext) *cobra.Command {
	return newProjectsLifecycleCommand(ctx, "stop", "Stop a Volcengine Supabase workspace", func(c *cobra.Command, id string) error {
		return projectdelete.RunVolcengineStop(c.Context(), id, afero.NewOsFs())
	})
}

func newProjectsSuspendCommand(ctx RuntimeContext) *cobra.Command {
	cmd := newProjectsLifecycleCommand(ctx, "sleep", "Sleep a Volcengine Supabase workspace", func(c *cobra.Command, id string) error {
		return projectdelete.RunVolcengineSuspend(c.Context(), id, afero.NewOsFs())
	})
	cmd.Aliases = []string{"suspend"}
	return cmd
}

func newProjectsWakeCommand(ctx RuntimeContext) *cobra.Command {
	return newProjectsLifecycleCommand(ctx, "wake", "Wake a suspended Volcengine Supabase workspace", func(c *cobra.Command, id string) error {
		return projectdelete.RunVolcengineWake(c.Context(), id, afero.NewOsFs())
	})
}

func newProjectsLifecycleCommand(ctx RuntimeContext, use, short string, run func(*cobra.Command, string) error) *cobra.Command {
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitWorkspaceMutation(cmd); err != nil {
			return err
		}
		_, err := resolveExplicitWorkspaceID(cmd, "Which project do you want to "+use+"?")
		return err
	}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := resolveExplicitWorkspaceID(cmd, "Which project do you want to "+use+"?")
		if err != nil {
			return err
		}
		return run(cmd, id)
	}}
	addExplicitWorkspaceFlags(cmd)
	_ = ctx
	return cmd
}

func addExplicitWorkspaceFlags(cmd *cobra.Command) {
	cmd.Flags().String("workspace-id", "", "Workspace ID to operate on.")
	cmd.Flags().String("project-ref", "", "Alias of --workspace-id.")
}

func requireExplicitWorkspaceMutation(cmd *cobra.Command) error {
	if _, err := resolveExplicitWorkspaceID(cmd, ""); err != nil {
		return err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) && !viper.GetBool("YES") {
		return errors.New("missing required flag: --yes")
	}
	return nil
}

func resolveExplicitWorkspaceID(cmd *cobra.Command, prompt string) (string, error) {
	workspaceID, _ := cmd.Flags().GetString("workspace-id")
	projectRef, _ := cmd.Flags().GetString("project-ref")
	workspaceID = strings.TrimSpace(workspaceID)
	projectRef = strings.TrimSpace(projectRef)
	if workspaceID != "" && projectRef != "" && workspaceID != projectRef {
		return "", errors.New("--workspace-id and --project-ref must match when both are supplied")
	}
	if workspaceID == "" {
		workspaceID = projectRef
	}
	if workspaceID == "" {
		return "", errors.New("missing required flag: --workspace-id (or --project-ref)")
	}
	_ = prompt
	return workspaceID, nil
}

func newProjectsRenameCommand(ctx RuntimeContext) *cobra.Command {
	var name string
	var workspaceID string
	cmd := &cobra.Command{Use: "rename", Short: "Rename a Volcengine Supabase workspace", Args: cobra.NoArgs, PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitWorkspaceMutation(cmd); err != nil {
			return err
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) && !cmd.Flags().Changed("name") {
			return errors.New("missing required flag: --name")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := resolveSettingsWorkspaceID(cmd, "Which project do you want to rename?")
		if err != nil {
			return err
		}
		if strings.TrimSpace(name) == "" {
			name, err = utils.NewConsole().PromptText(cmd.Context(), "Enter new project name: ")
			if err != nil {
				return err
			}
		}
		if strings.TrimSpace(name) == "" {
			return errors.New("missing required flag: --name")
		}
		return manage.RenameWorkspace(cmd.Context(), manage.RenameParams{WorkspaceID: id, WorkspaceName: name})
	}}
	cmd.Flags().StringVar(&name, "name", "", "New Volcengine Supabase workspace name.")
	cmd.Flags().StringVar(&workspaceID, "workspace-id", "", "Workspace ID to rename.")
	cmd.Flags().StringVar(&workspaceID, "project-ref", "", "Alias of --workspace-id.")
	_ = ctx
	return cmd
}

func newProjectsSettingsNameCommand(ctx RuntimeContext) *cobra.Command {
	cmd := newProjectsRenameCommand(ctx)
	cmd.Use = "rename"
	cmd.Short = "Rename a Volcengine Supabase workspace"
	cmd.Example = "byted-supabase-cli projects settings rename --workspace-id <workspace-id> --name <new-name>"
	return cmd
}

func newProjectsDeletionProtectionCommand(ctx RuntimeContext) *cobra.Command {
	var enable, disable bool
	cmd := &cobra.Command{Use: "deletion-protection", Short: "Modify deletion protection for a Volcengine Supabase workspace", Args: cobra.NoArgs, PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitWorkspaceMutation(cmd); err != nil {
			return err
		}
		if enable && disable {
			return errors.New("supply exactly one of --enable or --disable")
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) && !cmd.Flags().Changed("enable") && !cmd.Flags().Changed("disable") {
			return errors.New("supply exactly one of --enable or --disable")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := resolveSettingsWorkspaceID(cmd, "Which project do you want to modify deletion protection for?")
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("enable") && !cmd.Flags().Changed("disable") {
			enable, err = promptVolcengineDeletionProtection(cmd.Context())
			if err != nil {
				return err
			}
		}
		return manage.ModifyDeletionProtection(cmd.Context(), manage.DeletionProtectionParams{WorkspaceID: id, Enabled: enable})
	}}
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable deletion protection.")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable deletion protection.")
	addSettingsWorkspaceFlags(cmd)
	_ = ctx
	return cmd
}

func newProjectsSettingsDeletionProtectionCommand(ctx RuntimeContext) *cobra.Command {
	cmd := newProjectsDeletionProtectionCommand(ctx)
	cmd.Use = "deletion-protection"
	cmd.Short = "Configure workspace deletion protection"
	return cmd
}

func newProjectsCreateTagsCommand(ctx RuntimeContext) *cobra.Command {
	var tags []string
	cmd := &cobra.Command{Use: "create-tags [ref/workspace-id]", Short: "Create tags for a Volcengine Supabase workspace", Args: cobra.MaximumNArgs(1), PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := requireProjectsMutation(cmd, args); err != nil {
			return err
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) && len(tags) == 0 {
			return errors.New("missing required flag: --tag")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := resolveProjectsMutation(cmd, args, "Which project do you want to create tags for?")
		if err != nil {
			return err
		}
		if len(tags) == 0 {
			tags, err = promptVolcengineTags(cmd.Context())
			if err != nil {
				return err
			}
		}
		parsed, err := parseVolcengineTags(tags)
		if err != nil {
			return err
		}
		return manage.CreateTags(cmd.Context(), manage.TagsParams{WorkspaceID: id, Tags: parsed})
	}}
	cmd.Flags().StringArrayVar(&tags, "tag", []string{}, "Tag to create in key=value format. Can be specified multiple times.")
	_ = ctx
	return cmd
}

func newProjectsDeleteTagsCommand(ctx RuntimeContext) *cobra.Command {
	var keys []string
	cmd := &cobra.Command{Use: "delete-tags [ref/workspace-id]", Short: "Delete tags from a Volcengine Supabase workspace", Args: cobra.MaximumNArgs(1), PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := requireProjectsMutation(cmd, args); err != nil {
			return err
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) && len(keys) == 0 {
			return errors.New("missing required flag: --key")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := resolveProjectsMutation(cmd, args, "Which project do you want to delete tags from?")
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			keys, err = promptVolcengineTagKeys(cmd.Context(), id)
			if err != nil {
				return err
			}
		}
		parsed, err := parseVolcengineTagKeys(keys)
		if err != nil {
			return err
		}
		return manage.DeleteTags(cmd.Context(), manage.TagsParams{WorkspaceID: id, TagKeys: parsed})
	}}
	cmd.Flags().StringSliceVar(&keys, "key", []string{}, "Tag key to delete.")
	_ = ctx
	return cmd
}

func newProjectsWorkspaceSettingsCommand(ctx RuntimeContext) *cobra.Command {
	var retention int
	cmd := &cobra.Command{Use: "retention", Short: "Configure workspace history retention", Args: cobra.NoArgs, PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := requireExplicitWorkspaceMutation(cmd); err != nil {
			return err
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) && !cmd.Flags().Changed("history-retention-hours") {
			return errors.New("missing required flag: --history-retention-hours")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := resolveSettingsWorkspaceID(cmd, "Which project do you want to modify workspace settings for?")
		if err != nil {
			return err
		}
		if !cmd.Flags().Changed("history-retention-hours") {
			current, e := describeVolcengineWorkspaceForPrompt(cmd.Context(), id)
			if e != nil {
				return e
			}
			retention, err = promptVolcengineInt(cmd.Context(), "Enter HistoryRetentionHours", current.WorkspaceSetting.HistoryRetentionHours)
			if err != nil {
				return err
			}
		}
		return manage.ModifyWorkspaceSettings(cmd.Context(), manage.WorkspaceSettingsParams{WorkspaceID: id, HistoryRetentionHours: retention})
	}}
	cmd.Flags().IntVar(&retention, "history-retention-hours", 0, "History retention hours.")
	addSettingsWorkspaceFlags(cmd)
	_ = ctx
	return cmd
}

func newProjectsSettingsRetentionCommand(ctx RuntimeContext) *cobra.Command {
	cmd := newProjectsWorkspaceSettingsCommand(ctx)
	cmd.Use = "retention"
	cmd.Short = "Configure workspace history retention"
	return cmd
}

func newProjectsSettingsNetworkCommand(ctx RuntimeContext) *cobra.Command {
	var branchID, vpcID, subnetID, protocol string
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Configure workspace network protocol",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireExplicitWorkspaceMutation(cmd); err != nil {
				return err
			}
			if strings.TrimSpace(vpcID) == "" || strings.TrimSpace(subnetID) == "" {
				return errors.New("network settings require --vpc-id and --subnet-id")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveSettingsWorkspaceID(cmd, "Which project do you want to modify network settings for?")
			if err != nil {
				return err
			}
			cfgSet, err := volcengine.LoadConfigSetFromEnv()
			if err != nil {
				return err
			}
			workspace, cfg, err := findWorkspaceForSettings(cmd.Context(), cfgSet, id)
			if err != nil {
				return err
			}
			if err := volcengine.NewWriteClient(cfg).ModifyVpcSettings(cmd.Context(), volcengine.ModifyVpcSettingsParams{
				WorkspaceID: workspace.WorkspaceID, BranchID: branchID, VPCID: vpcID, SubnetID: subnetID, InternetProtocol: protocol,
			}); err != nil {
				return err
			}
			fmt.Printf("Configured network settings for workspace %s.\n", utils.Aqua(workspace.WorkspaceID))
			return nil
		},
	}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID to configure.")
	cmd.Flags().StringVar(&vpcID, "vpc-id", "", "VPC ID.")
	cmd.Flags().StringVar(&subnetID, "subnet-id", "", "Subnet ID.")
	cmd.Flags().StringVar(&protocol, "internet-protocol", "IPv4", "Internet protocol: IPv4 or DualStack.")
	addSettingsWorkspaceFlags(cmd)
	_ = ctx
	return cmd
}

func newProjectsSettingsPrivateDNSCommand(ctx RuntimeContext) *cobra.Command {
	var visibility bool
	cmd := &cobra.Command{
		Use:   "private-dns",
		Short: "Configure private DNS visibility",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := requireExplicitWorkspaceMutation(cmd); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := resolveSettingsWorkspaceID(cmd, "Which project do you want to modify private DNS visibility for?")
			if err != nil {
				return err
			}
			cfgSet, err := volcengine.LoadConfigSetFromEnv()
			if err != nil {
				return err
			}
			workspace, cfg, err := findWorkspaceForSettings(cmd.Context(), cfgSet, id)
			if err != nil {
				return err
			}
			if err := volcengine.NewWriteClient(cfg).ApplyPrivateDNSToPublic(cmd.Context(), workspace.WorkspaceID, visibility); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&visibility, "enabled", false, "Expose private DNS through public DNS.")
	addSettingsWorkspaceFlags(cmd)
	_ = ctx
	return cmd
}

func addSettingsWorkspaceFlags(cmd *cobra.Command) {
	cmd.Flags().String("workspace-id", "", "Workspace ID to configure.")
	cmd.Flags().String("project-ref", "", "Alias of --workspace-id.")
}

func resolveSettingsWorkspace(cmd *cobra.Command, options *projectsFactoryOptions, prompt string) error {
	id, err := resolveSettingsWorkspaceID(cmd, prompt)
	if err != nil {
		return err
	}
	options.WorkspaceID = id
	return nil
}

func resolveSettingsWorkspaceID(cmd *cobra.Command, prompt string) (string, error) {
	workspaceID, _ := cmd.Flags().GetString("workspace-id")
	projectRef, _ := cmd.Flags().GetString("project-ref")
	if strings.TrimSpace(workspaceID) != "" && strings.TrimSpace(projectRef) != "" && strings.TrimSpace(workspaceID) != strings.TrimSpace(projectRef) {
		return "", errors.New("--workspace-id and --project-ref must match when both are supplied")
	}
	if strings.TrimSpace(workspaceID) == "" {
		workspaceID = projectRef
	}
	options := &projectsFactoryOptions{WorkspaceID: strings.TrimSpace(workspaceID)}
	if err := resolveProjectsWorkspace(cmd, options, prompt); err != nil {
		return "", err
	}
	return options.WorkspaceID, nil
}

func findWorkspaceForSettings(ctx context.Context, cfgSet volcengine.ConfigSet, workspaceID string) (volcengine.Workspace, volcengine.Config, error) {
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		result, err := volcengine.NewClient(cfg).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		if result.Workspace.WorkspaceID == "" {
			continue
		}
		return result.Workspace, cfg, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, lastErr
	}
	return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("workspace %s not found", workspaceID)
}
