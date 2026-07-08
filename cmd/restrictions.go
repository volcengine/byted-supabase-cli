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
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/restrictions/manage"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	restrictionsCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "network-restrictions",
		Short:   "Manage network restrictions",
	}

	networkIPs []string
	appendMode bool
	deleteMode bool

	restrictionsGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get the current network restrictions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to get network restrictions for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return manage.Get(ctx, flags.ProjectRef)
		},
	}

	restrictionsCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create network restrictions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to create network restrictions for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			ipList, err := ensureNetworkRestrictionIPList(ctx)
			if err != nil {
				return err
			}
			return manage.Create(ctx, manage.Params{
				WorkspaceID: flags.ProjectRef,
				IPList:      ipList,
			})
		},
	}

	restrictionsUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update network restrictions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to update network restrictions for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			if appendMode && deleteMode {
				return errors.New("supply exactly one of --append or --delete")
			}
			ipList, err := ensureNetworkRestrictionIPList(ctx)
			if err != nil {
				return err
			}
			modifyMode := volcengine.ACLModifyModeCover
			if appendMode {
				modifyMode = volcengine.ACLModifyModeAppend
			} else if deleteMode {
				modifyMode = volcengine.ACLModifyModeDelete
			}
			return manage.Update(ctx, manage.Params{
				WorkspaceID: flags.ProjectRef,
				IPList:      ipList,
				ModifyMode:  modifyMode,
			})
		},
	}

	restrictionsDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete network restrictions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if flags.ProjectRef == "" {
				if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to delete network restrictions for?"); err != nil {
					return err
				}
			} else if err := ensureVolcengineRegion(ctx, fsys, false); err != nil {
				return err
			}
			return manage.Delete(ctx, flags.ProjectRef)
		},
	}
)

func init() {
	restrictionFlags := restrictionsCmd.PersistentFlags()
	restrictionFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(restrictionFlags.Lookup("project-ref"))
	restrictionFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	markFlagTelemetrySafe(restrictionFlags.Lookup("workspace-id"))
	addNetworkRestrictionIPFlags(restrictionsCreateCmd)
	updateFlags := restrictionsUpdateCmd.Flags()
	updateFlags.StringSliceVar(&networkIPs, "ip", []string{}, "IP or CIDR to allow DB connections from. Can be specified multiple times.")
	updateFlags.BoolVar(&appendMode, "append", false, "Append to existing restrictions instead of replacing them.")
	updateFlags.BoolVar(&deleteMode, "delete", false, "Delete IPs from existing restrictions.")
	restrictionsCmd.AddCommand(restrictionsGetCmd)
	restrictionsCmd.AddCommand(restrictionsCreateCmd)
	restrictionsCmd.AddCommand(restrictionsUpdateCmd)
	restrictionsCmd.AddCommand(restrictionsDeleteCmd)
	rootCmd.AddCommand(restrictionsCmd)
}

func addNetworkRestrictionIPFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringSliceVar(&networkIPs, "ip", []string{}, "IP or CIDR to allow DB connections from. Can be specified multiple times.")
}

func ensureNetworkRestrictionIPList(ctx context.Context) ([]string, error) {
	ipList := normalizeNetworkRestrictionIPs(networkIPs)
	if len(ipList) == 0 {
		console := utils.NewConsole()
		if !console.IsTTY {
			return nil, errors.New("missing IP list. Supply --ip.")
		}
		input, err := console.PromptText(ctx, "Enter IPs or CIDRs separated by comma: ")
		if err != nil {
			return nil, err
		}
		ipList = normalizeNetworkRestrictionIPs(strings.Split(input, ","))
	}
	if len(ipList) == 0 {
		return nil, errors.New("IP list cannot be empty")
	}
	if len(ipList) > 300 {
		return nil, errors.New("IP list cannot contain more than 300 entries")
	}
	return ipList, nil
}

func normalizeNetworkRestrictionIPs(values []string) []string {
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
