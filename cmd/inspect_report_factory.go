// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/inspect"
)

func newInspectReportCommand(ctx RuntimeContext) *cobra.Command {
	var options inspectOptions
	cmd := &cobra.Command{
		Use: "report", Short: "Generate a CSV output for all inspect commands",
		PreRunE: func(cmd *cobra.Command, args []string) error { return preRunInspectFactory(cmd, ctx.FS, &options) },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspectFactory(cmd, ctx.FS, &options, func(ctx context.Context, config pgconn.Config, fsys afero.Fs, connOptions ...func(*pgx.ConnConfig)) error {
				return inspect.Report(ctx, options.OutputDir, config, fsys, connOptions...)
			})
		},
	}
	addInspectTargetFlags(cmd, &options)
	cmd.Flags().StringVar(&options.OutputDir, "output-dir", ".", "Path to save CSV files in")
	return cmd
}
