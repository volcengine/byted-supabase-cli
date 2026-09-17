// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/restrictions/manage"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type networkRestrictionsOptions struct {
	WorkspaceID string
}

func networkRestrictionsCommandGroup() CommandGroup {
	return CommandGroup{Factories: []CommandFactory{newNetworkRestrictionsCommand}}
}

func newNetworkRestrictionsCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "network-restrictions",
		Short:   "Manage Volcengine Network Restrictions",
	}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newNetworkRestrictionsGetCommand(ctx),
		newNetworkRestrictionsCreateCommand(ctx),
		newNetworkRestrictionsUpdateCommand(ctx),
		newNetworkRestrictionsDeleteCommand(ctx),
	)
	return cmd
}

func addNetworkRestrictionsWorkspaceFlags(cmd *cobra.Command, options *networkRestrictionsOptions) {
	flags := cmd.Flags()
	flags.StringVar(&options.WorkspaceID, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("project-ref"))
	flags.StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(flags.Lookup("workspace-id"))
}

func prepareNetworkRestrictionsWorkspace(cmd *cobra.Command, fsys afero.Fs, options *networkRestrictionsOptions, prompt string) error {
	workspaceID, err := (WorkspaceResolver{FS: fsys}).Resolve(cmd.Context(), options.WorkspaceID, true)
	if err == nil {
		options.WorkspaceID = workspaceID
	}
	return err
}

func newNetworkRestrictionsGetCommand(ctx RuntimeContext) *cobra.Command {
	options := &networkRestrictionsOptions{}
	cmd := &cobra.Command{
		Use: "get", Short: "Get the current network restrictions", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareNetworkRestrictionsWorkspace(cmd, ctx.FS, options, "Which project do you want to get network restrictions for?"); err != nil {
				return err
			}
			return manage.Get(cmd.Context(), options.WorkspaceID)
		},
	}
	addNetworkRestrictionsWorkspaceFlags(cmd, options)
	return cmd
}

func newNetworkRestrictionsCreateCommand(ctx RuntimeContext) *cobra.Command {
	options := &networkRestrictionsOptions{}
	var ipList []string
	cmd := &cobra.Command{
		Use: "create", Short: "Create network restrictions", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareNetworkRestrictionsWorkspace(cmd, ctx.FS, options, "Which project do you want to create network restrictions for?"); err != nil {
				return err
			}
			ips, err := ensureNetworkRestrictionIPListFactory(cmd.Context(), ipList)
			if err != nil {
				return err
			}
			return manage.Create(cmd.Context(), manage.Params{WorkspaceID: options.WorkspaceID, IPList: ips})
		},
	}
	cmd.Flags().StringSliceVar(&ipList, "ip", []string{}, "IP or CIDR to allow DB connections from. Can be specified multiple times.")
	addNetworkRestrictionsWorkspaceFlags(cmd, options)
	return cmd
}

func newNetworkRestrictionsUpdateCommand(ctx RuntimeContext) *cobra.Command {
	options := &networkRestrictionsOptions{}
	var ipList []string
	var appendMode, deleteMode bool
	cmd := &cobra.Command{
		Use: "update", Short: "Update network restrictions", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareNetworkRestrictionsWorkspace(cmd, ctx.FS, options, "Which project do you want to update network restrictions for?"); err != nil {
				return err
			}
			if appendMode && deleteMode {
				return errors.New("supply exactly one of --append or --delete")
			}
			ips, err := ensureNetworkRestrictionIPListFactory(cmd.Context(), ipList)
			if err != nil {
				return err
			}
			modifyMode := volcengine.ACLModifyModeCover
			if appendMode {
				modifyMode = volcengine.ACLModifyModeAppend
			} else if deleteMode {
				modifyMode = volcengine.ACLModifyModeDelete
			}
			return manage.Update(cmd.Context(), manage.Params{WorkspaceID: options.WorkspaceID, IPList: ips, ModifyMode: modifyMode})
		},
	}
	cmd.Flags().StringSliceVar(&ipList, "ip", []string{}, "IP or CIDR to allow DB connections from. Can be specified multiple times.")
	cmd.Flags().BoolVar(&appendMode, "append", false, "Append to existing restrictions instead of replacing them.")
	cmd.Flags().BoolVar(&deleteMode, "delete", false, "Delete IPs from existing restrictions.")
	addNetworkRestrictionsWorkspaceFlags(cmd, options)
	return cmd
}

func newNetworkRestrictionsDeleteCommand(ctx RuntimeContext) *cobra.Command {
	options := &networkRestrictionsOptions{}
	cmd := &cobra.Command{
		Use: "delete", Short: "Delete network restrictions", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareNetworkRestrictionsWorkspace(cmd, ctx.FS, options, "Which project do you want to delete network restrictions for?"); err != nil {
				return err
			}
			return manage.Delete(cmd.Context(), options.WorkspaceID)
		},
	}
	addNetworkRestrictionsWorkspaceFlags(cmd, options)
	return cmd
}

func ensureNetworkRestrictionIPListFactory(ctx context.Context, values []string) ([]string, error) {
	ipList := normalizeNetworkRestrictionIPsFactory(values)
	if len(ipList) == 0 {
		console := utils.NewConsole()
		if !console.IsTTY {
			return nil, errors.New("missing IP list. Supply --ip.")
		}
		input, err := console.PromptText(ctx, "Enter IPs or CIDRs separated by comma: ")
		if err != nil {
			return nil, err
		}
		ipList = normalizeNetworkRestrictionIPsFactory(strings.Split(input, ","))
	}
	if len(ipList) == 0 {
		return nil, errors.New("IP list cannot be empty")
	}
	if len(ipList) > 300 {
		return nil, errors.New("IP list cannot contain more than 300 entries")
	}
	return ipList, nil
}

func normalizeNetworkRestrictionIPsFactory(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
