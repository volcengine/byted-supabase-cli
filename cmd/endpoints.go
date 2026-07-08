// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	endpointeips "github.com/volcengine/byted-supabase-cli/internal/endpoints/eips"
	endpointlist "github.com/volcengine/byted-supabase-cli/internal/endpoints/list"
	endpointmanage "github.com/volcengine/byted-supabase-cli/internal/endpoints/manage"
	endpointnetwork "github.com/volcengine/byted-supabase-cli/internal/endpoints/network"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
)

var (
	endpointsBranchID         string
	endpointsEndpointID       string
	endpointsEIPID            string
	endpointsEIPsAll          bool
	endpointsListLimit        int
	endpointsVPCID            string
	endpointsSubnetID         string
	endpointsInternetProtocol string

	endpointsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "endpoints",
		Short:   "Manage Volcengine Supabase workspace endpoints",
	}

	endpointsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List workspace endpoints and connection addresses",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to list endpoints for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return endpointlist.RunVolcengine(ctx, endpointlist.RunVolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    endpointsBranchID,
			})
		},
	}

	endpointsEnablePublicCmd = &cobra.Command{
		Use:   "enable-public",
		Short: "Enable public endpoint access for a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to enable public endpoint access for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return endpointmanage.EnablePublic(ctx, endpointmanage.EnablePublicParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    endpointsBranchID,
				EndpointID:  endpointsEndpointID,
				EIPID:       endpointsEIPID,
			})
		},
	}

	endpointsDisablePublicCmd = &cobra.Command{
		Use:   "disable-public",
		Short: "Disable public endpoint access for a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to disable public endpoint access for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return endpointmanage.DisablePublic(ctx, endpointmanage.DisablePublicParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    endpointsBranchID,
				EndpointID:  endpointsEndpointID,
			})
		},
	}

	endpointsEnablePrivateCmd = &cobra.Command{
		Use:   "enable-private",
		Short: "Enable private endpoint access for a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to enable private endpoint access for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return endpointmanage.EnablePrivate(ctx, endpointmanage.EnablePrivateParams{
				WorkspaceID:      flags.ProjectRef,
				BranchID:         endpointsBranchID,
				VPCID:            endpointsVPCID,
				SubnetID:         endpointsSubnetID,
				InternetProtocol: endpointsInternetProtocol,
			})
		},
	}

	endpointsEIPsCmd = &cobra.Command{
		Use:   "eips",
		Short: "Query EIPs available for dedicated public endpoint access",
	}

	endpointsEIPsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List EIPs in the selected Volcengine region",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			if err := validateVolcenginePageFlags(endpointsListLimit, 0, false); err != nil {
				return err
			}
			return endpointeips.List(cmd.Context(), endpointsEIPsAll, endpointsListLimit)
		},
	}

	endpointsVPCsCmd = &cobra.Command{
		Use:   "vpcs",
		Short: "Query VPCs available for private endpoint access",
	}

	endpointsVPCsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List VPCs in the selected Volcengine region",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			if err := validateVolcenginePageFlags(endpointsListLimit, 0, false); err != nil {
				return err
			}
			return endpointnetwork.ListVPCs(cmd.Context(), endpointsListLimit)
		},
	}

	endpointsSubnetsCmd = &cobra.Command{
		Use:     "subnets",
		Short:   "Query subnets available for private endpoint access",
		Example: "byted-supabase-cli endpoints subnets list --vpc-id <vpc-id>",
	}

	endpointsSubnetsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List subnets in a VPC",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if endpointsVPCID == "" {
				return errors.New("missing required flag: --vpc-id")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			if err := validateVolcenginePageFlags(endpointsListLimit, 0, false); err != nil {
				return err
			}
			return endpointnetwork.ListSubnets(cmd.Context(), endpointsVPCID, endpointsListLimit)
		},
	}
)

func init() {
	addEndpointWorkspaceFlags(endpointsListCmd)
	endpointsListCmd.Flags().StringVar(&endpointsBranchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	addEndpointWorkspaceFlags(endpointsEnablePublicCmd)
	enablePublicFlags := endpointsEnablePublicCmd.Flags()
	enablePublicFlags.StringVar(&endpointsBranchID, "branch-id", "", "Branch ID to expose. Defaults to the workspace default branch.")
	enablePublicFlags.StringVar(&endpointsEndpointID, "endpoint-id", "", "Endpoint ID for dedicated public access. Omit to enable shared public access.")
	enablePublicFlags.StringVar(&endpointsEIPID, "eip-id", "", "EIP allocation ID for dedicated public access. Requires --endpoint-id.")
	addEndpointWorkspaceFlags(endpointsDisablePublicCmd)
	disablePublicFlags := endpointsDisablePublicCmd.Flags()
	disablePublicFlags.StringVar(&endpointsBranchID, "branch-id", "", "Branch ID to update. Defaults to the workspace default branch.")
	disablePublicFlags.StringVar(&endpointsEndpointID, "endpoint-id", "", "Endpoint ID for dedicated public access. Omit to disable shared public access.")
	addEndpointWorkspaceFlags(endpointsEnablePrivateCmd)
	enablePrivateFlags := endpointsEnablePrivateCmd.Flags()
	enablePrivateFlags.StringVar(&endpointsBranchID, "branch-id", "", "Branch ID to expose. Defaults to the workspace default branch.")
	enablePrivateFlags.StringVar(&endpointsVPCID, "vpc-id", "", "VPC ID for private endpoint access.")
	enablePrivateFlags.StringVar(&endpointsSubnetID, "subnet-id", "", "Subnet ID for private endpoint access.")
	enablePrivateFlags.StringVar(&endpointsInternetProtocol, "internet-protocol", "IPv4", "Private endpoint protocol: IPv4 or DualStack.")
	endpointsEIPsListCmd.Flags().BoolVar(&endpointsEIPsAll, "all", false, "List EIPs in all statuses instead of only available EIPs.")
	endpointsEIPsListCmd.Flags().IntVar(&endpointsListLimit, "limit", volcengineDefaultListLimit, "Maximum number of EIPs to return (1-100).")
	endpointsVPCsListCmd.Flags().IntVar(&endpointsListLimit, "limit", volcengineDefaultListLimit, "Maximum number of VPCs to return (1-100).")
	endpointsSubnetsListCmd.Flags().StringVar(&endpointsVPCID, "vpc-id", "", "VPC ID whose subnets should be listed.")
	endpointsSubnetsListCmd.Flags().IntVar(&endpointsListLimit, "limit", volcengineDefaultListLimit, "Maximum number of subnets to return (1-100).")
	endpointsEIPsCmd.AddCommand(endpointsEIPsListCmd)
	endpointsVPCsCmd.AddCommand(endpointsVPCsListCmd)
	endpointsSubnetsCmd.AddCommand(endpointsSubnetsListCmd)
	endpointsCmd.AddCommand(endpointsListCmd)
	endpointsCmd.AddCommand(endpointsEnablePublicCmd)
	endpointsCmd.AddCommand(endpointsDisablePublicCmd)
	endpointsCmd.AddCommand(endpointsEnablePrivateCmd)
	endpointsCmd.AddCommand(endpointsEIPsCmd)
	endpointsCmd.AddCommand(endpointsVPCsCmd)
	endpointsCmd.AddCommand(endpointsSubnetsCmd)
	rootCmd.AddCommand(endpointsCmd)
}

func addEndpointWorkspaceFlags(cmd *cobra.Command) {
	endpointFlags := cmd.Flags()
	endpointFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(endpointFlags.Lookup("project-ref"))
	endpointFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(endpointFlags.Lookup("workspace-id"))
}
