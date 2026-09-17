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
)

type endpointsOptions struct {
	WorkspaceID string
}

func endpointsCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newEndpointsCommand}}
}

func newEndpointsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "endpoints",
		Short:   "Manage Volcengine Supabase Workspace Endpoints",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newEndpointsListCommand(ctx),
		newEndpointsEnablePublicCommand(ctx),
		newEndpointsDisablePublicCommand(ctx),
		newEndpointsEnablePrivateCommand(ctx),
		newEndpointsEIPsCommand(ctx),
		newEndpointsVPCsCommand(ctx),
		newEndpointsSubnetsCommand(ctx),
	)
	return cmd
}

func addEndpointsWorkspaceFlags(cmd *cobra.Command, options *endpointsOptions) {
	flags := cmd.Flags()
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func prepareEndpointsWorkspace(cmd *cobra.Command, fsys afero.Fs, options *endpointsOptions, prompt string) error {
	workspaceID, err := (WorkspaceResolver{FS: fsys}).Resolve(cmd.Context(), options.WorkspaceID, true)
	if err == nil {
		options.WorkspaceID = workspaceID
	}
	return err
}

func newEndpointsListCommand(ctx RuntimeContext) *cobra.Command {
	parent := &endpointsOptions{}
	var branchID string
	cmd := &cobra.Command{
		Use: "list", Short: "List workspace endpoints and connection addresses", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareEndpointsWorkspace(cmd, ctx.FS, parent, "Which project do you want to list endpoints for?"); err != nil {
				return err
			}
			return endpointlist.RunVolcengine(cmd.Context(), endpointlist.RunVolcengineParams{
				WorkspaceID: parent.WorkspaceID, BranchID: branchID,
			})
		},
	}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID to query. Defaults to the workspace default branch.")
	addEndpointsWorkspaceFlags(cmd, parent)
	return cmd
}

func newEndpointsEnablePublicCommand(ctx RuntimeContext) *cobra.Command {
	parent := &endpointsOptions{}
	var branchID, endpointID, eipID string
	cmd := &cobra.Command{
		Use: "enable-public", Short: "Enable public endpoint access for a branch", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareEndpointsWorkspace(cmd, ctx.FS, parent, "Which project do you want to enable public endpoint access for?"); err != nil {
				return err
			}
			return endpointmanage.EnablePublic(cmd.Context(), endpointmanage.EnablePublicParams{
				WorkspaceID: parent.WorkspaceID, BranchID: branchID, EndpointID: endpointID, EIPID: eipID,
			})
		},
	}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID to expose. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&endpointID, "endpoint-id", "", "Endpoint ID for dedicated public access. Omit to enable shared public access.")
	cmd.Flags().StringVar(&eipID, "eip-id", "", "EIP allocation ID for dedicated public access. Requires --endpoint-id.")
	addEndpointsWorkspaceFlags(cmd, parent)
	return cmd
}

func newEndpointsDisablePublicCommand(ctx RuntimeContext) *cobra.Command {
	parent := &endpointsOptions{}
	var branchID, endpointID string
	cmd := &cobra.Command{
		Use: "disable-public", Short: "Disable public endpoint access for a branch", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareEndpointsWorkspace(cmd, ctx.FS, parent, "Which project do you want to disable public endpoint access for?"); err != nil {
				return err
			}
			return endpointmanage.DisablePublic(cmd.Context(), endpointmanage.DisablePublicParams{
				WorkspaceID: parent.WorkspaceID, BranchID: branchID, EndpointID: endpointID,
			})
		},
	}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID to update. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&endpointID, "endpoint-id", "", "Endpoint ID for dedicated public access. Omit to disable shared public access.")
	addEndpointsWorkspaceFlags(cmd, parent)
	return cmd
}

func newEndpointsEnablePrivateCommand(ctx RuntimeContext) *cobra.Command {
	parent := &endpointsOptions{}
	var branchID, vpcID, subnetID, internetProtocol string
	cmd := &cobra.Command{
		Use: "enable-private", Short: "Enable private endpoint access for a branch", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareEndpointsWorkspace(cmd, ctx.FS, parent, "Which project do you want to enable private endpoint access for?"); err != nil {
				return err
			}
			return endpointmanage.EnablePrivate(cmd.Context(), endpointmanage.EnablePrivateParams{
				WorkspaceID: parent.WorkspaceID, BranchID: branchID, VPCID: vpcID, SubnetID: subnetID, InternetProtocol: internetProtocol,
			})
		},
	}
	cmd.Flags().StringVar(&branchID, "branch-id", "", "Branch ID to expose. Defaults to the workspace default branch.")
	cmd.Flags().StringVar(&vpcID, "vpc-id", "", "VPC ID for private endpoint access.")
	cmd.Flags().StringVar(&subnetID, "subnet-id", "", "Subnet ID for private endpoint access.")
	cmd.Flags().StringVar(&internetProtocol, "internet-protocol", "IPv4", "Private endpoint protocol: IPv4 or DualStack.")
	addEndpointsWorkspaceFlags(cmd, parent)
	return cmd
}

func newEndpointsEIPsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "eips", Short: "Query EIPs available for dedicated public endpoint access"}
	cmd.AddCommand(newEndpointsEIPsListCommand(ctx))
	return cmd
}

func newEndpointsEIPsListCommand(ctx RuntimeContext) *cobra.Command {
	var all bool
	var limit int
	cmd := newEndpointsRegionalListCommand("list", "List EIPs in the selected Volcengine region", &limit, func(cmd *cobra.Command) error {
		return endpointeips.List(cmd.Context(), all, limit)
	})
	cmd.Flags().BoolVar(&all, "all", false, "List EIPs in all statuses instead of only available EIPs.")
	return cmd
}

func newEndpointsVPCsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "vpcs", Short: "Query VPCs available for private endpoint access"}
	cmd.AddCommand(newEndpointsVPCsListCommand(ctx))
	return cmd
}

func newEndpointsVPCsListCommand(ctx RuntimeContext) *cobra.Command {
	var limit int
	return newEndpointsRegionalListCommand("list", "List VPCs in the selected Volcengine region", &limit, func(cmd *cobra.Command) error {
		return endpointnetwork.ListVPCs(cmd.Context(), limit)
	})
}

func newEndpointsSubnetsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use: "subnets", Short: "Query subnets available for private endpoint access",
		Example: "byted-supabase-cli endpoints subnets list --vpc-id <vpc-id>",
	}
	cmd.AddCommand(newEndpointsSubnetsListCommand(ctx))
	return cmd
}

func newEndpointsSubnetsListCommand(ctx RuntimeContext) *cobra.Command {
	var vpcID string
	var limit int
	cmd := newEndpointsRegionalListCommand("list", "List subnets in a VPC", &limit, func(cmd *cobra.Command) error {
		return endpointnetwork.ListSubnets(cmd.Context(), vpcID, limit)
	})
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if vpcID == "" {
			return errors.New("missing required flag: --vpc-id")
		}
		return nil
	}
	cmd.Flags().StringVar(&vpcID, "vpc-id", "", "VPC ID whose subnets should be listed.")
	return cmd
}

func newEndpointsRegionalListCommand(use, short string, limit *int, run func(*cobra.Command) error) *cobra.Command {
	cmd := &cobra.Command{
		Use: use, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), afero.NewOsFs(), false); err != nil {
				return err
			}
			if err := validateVolcenginePageFlags(*limit, 0, false); err != nil {
				return err
			}
			return run(cmd)
		},
	}
	cmd.Flags().IntVar(limit, "limit", volcengineDefaultListLimit, "Maximum number of resources to return (1-100).")
	return cmd
}
