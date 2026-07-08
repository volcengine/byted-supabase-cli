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
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/seed/buckets"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	seedCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "seed",
		Short:   "Seed a Supabase project from " + utils.ConfigPath,
	}

	seedBucketsBranchID string

	bucketsCmd = &cobra.Command{
		Use:   "buckets",
		Short: "Seed buckets declared in [storage.buckets]",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineStorage(cmd, "Which project do you want to seed Storage buckets for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fsys := afero.NewOsFs()
			if err := flags.LoadConfig(fsys); err != nil {
				return err
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return buckets.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, seedBucketsBranchID, true, fsys)
		},
	}
)

func init() {
	seedFlags := seedCmd.PersistentFlags()
	seedFlags.Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	// Volcengine seed buckets targets remote Storage endpoints only:
	// seedFlags.Bool("local", true, "Seeds the local database.")
	// seedCmd.MarkFlagsMutuallyExclusive("local", "linked")
	seedFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	seedFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	seedFlags.StringVar(&seedBucketsBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	seedCmd.AddCommand(bucketsCmd)
	rootCmd.AddCommand(seedCmd)
}
