// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"

	"os"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	computedescribe "github.com/volcengine/byted-supabase-cli/internal/computes/describe"
	computemanage "github.com/volcengine/byted-supabase-cli/internal/computes/manage"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

var (
	computesBranchID    string
	computesServiceType string
	computesName        string
	computesMinCU       float64
	computesMaxCU       float64

	computesCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "computes",
		Short:   "Manage Volcengine computes in a Supabase workspace",
	}

	computesListCmd = &cobra.Command{
		Use:   "list",
		Short: "List computes for a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			serviceType, err := normalizeComputeServiceType(computesServiceType)
			if err != nil {
				return err
			}
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to list computes for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return computedescribe.RunList(ctx, computedescribe.ListParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    computesBranchID,
				ServiceType: serviceType,
			})
		},
	}

	computesGetCmd = &cobra.Command{
		Use:   "get [compute-id]",
		Short: "Retrieve details of a compute",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to get compute details for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			computeID := ""
			if len(args) > 0 {
				computeID = strings.TrimSpace(args[0])
			} else {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return errors.New("missing compute id. Supply computes get [compute-id].")
				}
				selected, err := computedescribe.PromptComputeID(ctx, computedescribe.ListParams{
					WorkspaceID: flags.ProjectRef,
					BranchID:    computesBranchID,
				})
				if err != nil {
					return err
				}
				computeID = selected
			}
			return computedescribe.RunGet(ctx, computedescribe.GetParams{
				WorkspaceID: flags.ProjectRef,
				ComputeID:   computeID,
			})
		},
	}

	computesUpdateCmd = &cobra.Command{
		Use:   "update [compute-id]",
		Short: "Update compute name or autoscaling specification",
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("name") && strings.TrimSpace(computesName) == "" {
				return errors.New("compute name cannot be empty")
			}
			modifySpec := cmd.Flags().Changed("min-cu") || cmd.Flags().Changed("max-cu")
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				if len(args) == 0 {
					return errors.New("missing compute id. Supply computes update [compute-id].")
				}
				if strings.TrimSpace(computesName) == "" && !modifySpec {
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
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to update a compute for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			computeID := ""
			if len(args) > 0 {
				computeID = strings.TrimSpace(args[0])
			} else {
				selected, err := computedescribe.PromptComputeID(ctx, computedescribe.ListParams{
					WorkspaceID: flags.ProjectRef,
					BranchID:    computesBranchID,
				})
				if err != nil {
					return err
				}
				computeID = selected
			}
			modifySpec := cmd.Flags().Changed("min-cu") || cmd.Flags().Changed("max-cu") || strings.TrimSpace(computesName) == ""
			if modifySpec && (!cmd.Flags().Changed("min-cu") || !cmd.Flags().Changed("max-cu")) {
				current, err := computedescribe.GetCompute(ctx, computedescribe.GetParams{
					WorkspaceID: flags.ProjectRef,
					ComputeID:   computeID,
				})
				if err != nil {
					return err
				}
				if !cmd.Flags().Changed("min-cu") {
					computesMinCU, err = promptVolcengineFloat64(ctx, "Enter AutoScalingLimitMinCU", current.AutoScalingLimitMinCU)
					if err != nil {
						return err
					}
				}
				if !cmd.Flags().Changed("max-cu") {
					computesMaxCU, err = promptVolcengineFloat64(ctx, "Enter AutoScalingLimitMaxCU", current.AutoScalingLimitMaxCU)
					if err != nil {
						return err
					}
				}
			}
			return computemanage.Update(ctx, computemanage.UpdateParams{
				WorkspaceID:           flags.ProjectRef,
				BranchID:              computesBranchID,
				ComputeID:             computeID,
				ComputeName:           strings.TrimSpace(computesName),
				AutoScalingLimitMinCU: computesMinCU,
				AutoScalingLimitMaxCU: computesMaxCU,
				ModifySpec:            modifySpec,
			})
		},
	}
)

func init() {
	computeFlags := computesCmd.PersistentFlags()
	computeFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(computeFlags.Lookup("project-ref"))
	computeFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(computeFlags.Lookup("workspace-id"))
	computesListCmd.Flags().StringVar(&computesBranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	computesListCmd.Flags().StringVar(&computesServiceType, "service-type", "", "Optional compute service type filter: Supabase or Database.")
	computesGetCmd.Flags().StringVar(&computesBranchID, "branch-id", "", "Branch ID used when interactively selecting a compute. Defaults to the workspace default branch.")
	computesUpdateCmd.Flags().StringVar(&computesBranchID, "branch-id", "", "Branch ID used when interactively selecting a compute. Defaults to the workspace default branch.")
	computesUpdateCmd.Flags().StringVar(&computesName, "name", "", "New compute name.")
	computesUpdateCmd.Flags().Float64Var(&computesMinCU, "min-cu", 0, "Minimum compute CU for autoscaling.")
	computesUpdateCmd.Flags().Float64Var(&computesMaxCU, "max-cu", 0, "Maximum compute CU for autoscaling.")
	computesCmd.AddCommand(computesListCmd)
	computesCmd.AddCommand(computesGetCmd)
	computesCmd.AddCommand(computesUpdateCmd)
	rootCmd.AddCommand(computesCmd)
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
