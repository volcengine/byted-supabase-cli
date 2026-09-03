// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	computedescribe "github.com/volcengine/byted-supabase-cli/internal/computes/describe"
	computemanage "github.com/volcengine/byted-supabase-cli/internal/computes/manage"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

type computesOptions struct {
	WorkspaceID string
}

func computesCommandGroup() CommandGroup {
	return CommandGroup{
		Factories: []CommandFactory{newComputesCommand},
	}
}

func newComputesCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "computes",
		Short:   "Manage Volcengine Computes in a Supabase Workspace",
	}
	cmd.AddCommand(
		newComputesListCommand(ctx),
		newComputesGetCommand(ctx),
		newComputesUpdateCommand(ctx),
	)
	return cmd
}

func addComputesWorkspaceFlags(cmd *cobra.Command, options *computesOptions) {
	flags := cmd.Flags()
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func prepareComputesWorkspace(cmd *cobra.Command, fsys afero.Fs, options *computesOptions, prompt string) error {
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

func newComputesListCommand(ctx RuntimeContext) *cobra.Command {
	parent := &computesOptions{}
	var options struct {
		BranchID    string
		ServiceType string
	}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List computes for a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceType, err := normalizeComputeServiceType(options.ServiceType)
			if err != nil {
				return err
			}
			if err := prepareComputesWorkspace(cmd, ctx.FS, parent, "Which project do you want to list computes for?"); err != nil {
				return err
			}
			return computedescribe.RunList(cmd.Context(), computedescribe.ListParams{
				WorkspaceID: parent.WorkspaceID,
				BranchID:    options.BranchID,
				ServiceType: serviceType,
			})
		},
	}
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&options.ServiceType, "service-type", "", "Optional compute service type filter: Supabase or Database.")
	addComputesWorkspaceFlags(cmd, parent)
	return cmd
}

func newComputesGetCommand(ctx RuntimeContext) *cobra.Command {
	parent := &computesOptions{}
	var options struct{ BranchID string }
	var computeID string
	cmd := &cobra.Command{
		Use:   "get [compute-id]",
		Short: "Retrieve details of a compute",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareComputesWorkspace(cmd, ctx.FS, parent, "Which project do you want to get compute details for?"); err != nil {
				return err
			}
			computeID = ""
			if len(args) > 0 {
				computeID = strings.TrimSpace(args[0])
			} else {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return errors.New("missing compute id. Supply computes get [compute-id].")
				}
				selected, err := computedescribe.PromptComputeID(cmd.Context(), computedescribe.ListParams{
					WorkspaceID: parent.WorkspaceID,
					BranchID:    options.BranchID,
				})
				if err != nil {
					return err
				}
				computeID = selected
			}
			return computedescribe.RunGet(cmd.Context(), computedescribe.GetParams{
				WorkspaceID: parent.WorkspaceID,
				ComputeID:   computeID,
			})
		},
	}
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID used when interactively selecting a compute. Defaults to the workspace default branch.")
	addComputesWorkspaceFlags(cmd, parent)
	return cmd
}

func newComputesUpdateCommand(ctx RuntimeContext) *cobra.Command {
	parent := &computesOptions{}
	var options struct {
		BranchID string
		Name     string
		MinCU    float64
		MaxCU    float64
	}
	var computeID string
	cmd := &cobra.Command{
		Use:   "update [compute-id]",
		Short: "Update compute name or autoscaling specification",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("name") && strings.TrimSpace(options.Name) == "" {
				return errors.New("compute name cannot be empty")
			}
			modifySpec := cmd.Flags().Changed("min-cu") || cmd.Flags().Changed("max-cu")
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				if len(args) == 0 {
					return errors.New("missing compute id. Supply computes update [compute-id].")
				}
				if strings.TrimSpace(options.Name) == "" && !modifySpec {
					return errors.New("supply at least one update: --name, --min-cu, or --max-cu")
				}
				if modifySpec && !cmd.Flags().Changed("min-cu") {
					return errors.New("missing required flag: --min-cu")
				}
				if modifySpec && !cmd.Flags().Changed("max-cu") {
					return errors.New("missing required flag: --max-cu")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareComputesWorkspace(cmd, ctx.FS, parent, "Which project do you want to update a compute for?"); err != nil {
				return err
			}
			computeID = ""
			if len(args) > 0 {
				computeID = strings.TrimSpace(args[0])
			} else {
				selected, err := computedescribe.PromptComputeID(cmd.Context(), computedescribe.ListParams{
					WorkspaceID: parent.WorkspaceID,
					BranchID:    options.BranchID,
				})
				if err != nil {
					return err
				}
				computeID = selected
			}
			modifySpec := cmd.Flags().Changed("min-cu") || cmd.Flags().Changed("max-cu") || strings.TrimSpace(options.Name) == ""
			if modifySpec && (!cmd.Flags().Changed("min-cu") || !cmd.Flags().Changed("max-cu")) {
				current, err := computedescribe.GetCompute(cmd.Context(), computedescribe.GetParams{
					WorkspaceID: parent.WorkspaceID,
					ComputeID:   computeID,
				})
				if err != nil {
					return err
				}
				if !cmd.Flags().Changed("min-cu") {
					options.MinCU, err = promptVolcengineFloat64(cmd.Context(), "Enter AutoScalingLimitMinCU", current.AutoScalingLimitMinCU)
					if err != nil {
						return err
					}
				}
				if !cmd.Flags().Changed("max-cu") {
					options.MaxCU, err = promptVolcengineFloat64(cmd.Context(), "Enter AutoScalingLimitMaxCU", current.AutoScalingLimitMaxCU)
					if err != nil {
						return err
					}
				}
			}
			return computemanage.Update(cmd.Context(), computemanage.UpdateParams{
				WorkspaceID:           parent.WorkspaceID,
				BranchID:              options.BranchID,
				ComputeID:             computeID,
				ComputeName:           strings.TrimSpace(options.Name),
				AutoScalingLimitMinCU: options.MinCU,
				AutoScalingLimitMaxCU: options.MaxCU,
				ModifySpec:            modifySpec,
			})
		},
	}
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID used when interactively selecting a compute. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&options.Name, "name", "", "New compute name.")
	cmd.Flags().Float64Var(&options.MinCU, "min-cu", 0, "Minimum compute CU for autoscaling.")
	cmd.Flags().Float64Var(&options.MaxCU, "max-cu", 0, "Maximum compute CU for autoscaling.")
	addComputesWorkspaceFlags(cmd, parent)
	return cmd
}

func normalizeComputeServiceType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "", nil
	case "supabase":
		return volcengine.ServiceTypeSupabase, nil
	case "database":
		return volcengine.ServiceTypeDatabase, nil
	default:
		return "", errors.New("invalid --service-type. Expected Supabase or Database.")
	}
}
