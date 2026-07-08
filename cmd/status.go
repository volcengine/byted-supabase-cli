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
	env "github.com/Netflix/go-env"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/status"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

var (
	override []string
	names    status.CustomName

	statusCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "status",
		Short:   "Show status of local Supabase containers",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			es, err := env.EnvironToEnvSet(override)
			if err != nil {
				return err
			}
			return env.Unmarshal(es, &names)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return status.Run(cmd.Context(), names, utils.OutputFormat.Value, afero.NewOsFs())
		},
		Example: `  byted-supabase-cli status -o env --override-name api.url=NEXT_PUBLIC_SUPABASE_URL
  byted-supabase-cli status -o json`,
	}
)

func init() {
	flags := statusCmd.Flags()
	flags.StringSliceVar(&override, "override-name", []string{}, "Override specific variable names.")
	// Volcengine CLI does not support Supabase local stack status in the current phase.
	// Keep the original command registered code here for future local development support.
	// rootCmd.AddCommand(statusCmd)
}
