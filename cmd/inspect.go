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

	"github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	dbconnection "github.com/volcengine/byted-supabase-cli/internal/db/connection"
	"github.com/volcengine/byted-supabase-cli/internal/inspect"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/bloat"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/blocking"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/calls"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/db_stats"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/index_stats"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/locks"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/long_running_queries"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/outliers"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/replication_slots"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/role_stats"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/table_stats"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/traffic_profile"
	"github.com/volcengine/byted-supabase-cli/internal/inspect/vacuum_stats"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	inspectCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "inspect",
		Short:   "Tools to inspect your Supabase project",
	}

	inspectDBCmd = &cobra.Command{
		Use:   "db",
		Short: "Tools to inspect your Supabase database",
	}

	inspectDBStatsCmd = &cobra.Command{
		Use:     "db-stats",
		Short:   "Show stats such as cache hit rates, total sizes, and WAL size",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, db_stats.Run)
		},
	}

	inspectReplicationSlotsCmd = &cobra.Command{
		Use:     "replication-slots",
		Short:   "Show information about replication slots on the database",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, replication_slots.Run)
		},
	}

	inspectLocksCmd = &cobra.Command{
		Use:     "locks",
		Short:   "Show queries which have taken out an exclusive lock on a relation",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, locks.Run)
		},
	}

	inspectBlockingCmd = &cobra.Command{
		Use:     "blocking",
		Short:   "Show queries that are holding locks and the queries that are waiting for them to be released",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, blocking.Run)
		},
	}

	inspectOutliersCmd = &cobra.Command{
		Use:     "outliers",
		Short:   "Show queries from pg_stat_statements ordered by total execution time",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, outliers.Run)
		},
	}

	inspectCallsCmd = &cobra.Command{
		Use:     "calls",
		Short:   "Show queries from pg_stat_statements ordered by total times called",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, calls.Run)
		},
	}

	inspectIndexStatsCmd = &cobra.Command{
		Use:     "index-stats",
		Short:   "Show combined index size, usage percent, scan counts, and unused status",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, index_stats.Run)
		},
	}

	inspectLongRunningQueriesCmd = &cobra.Command{
		Use:     "long-running-queries",
		Short:   "Show currently running queries running for longer than 5 minutes",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, long_running_queries.Run)
		},
	}

	inspectBloatCmd = &cobra.Command{
		Use:     "bloat",
		Short:   "Estimates space allocated to a relation that is full of dead tuples",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, bloat.Run)
		},
	}

	inspectRoleStatsCmd = &cobra.Command{
		Use:     "role-stats",
		Short:   "Show information about roles on the database",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, role_stats.Run)
		},
	}

	inspectVacuumStatsCmd = &cobra.Command{
		Use:     "vacuum-stats",
		Short:   "Show statistics related to vacuum operations per table",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, vacuum_stats.Run)
		},
	}

	inspectTableStatsCmd = &cobra.Command{
		Use:     "table-stats",
		Short:   "Show combined table size, index size, and estimated row count",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, table_stats.Run)
		},
	}

	inspectTrafficProfileCmd = &cobra.Command{
		Use:     "traffic-profile",
		Short:   "Show read/write activity ratio for tables based on block I/O operations",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, traffic_profile.Run)
		},
	}

	inspectCacheHitCmd = &cobra.Command{
		Deprecated: `use "db-stats" instead.`,
		Use:        "cache-hit",
		Short:      "Show cache hit rates for tables and indices",
		RunE: func(cmd *cobra.Command, args []string) error {
			return db_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectIndexUsageCmd = &cobra.Command{
		Deprecated: `use "index-stats" instead.`,
		Use:        "index-usage",
		Short:      "Show information about the efficiency of indexes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return index_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectTotalIndexSizeCmd = &cobra.Command{
		Deprecated: `use "index-stats" instead.`,
		Use:        "total-index-size",
		Short:      "Show total size of all indexes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return index_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectIndexSizesCmd = &cobra.Command{
		Deprecated: `use "index-stats" instead.`,
		Use:        "index-sizes",
		Short:      "Show index sizes of individual indexes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return index_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectTableSizesCmd = &cobra.Command{
		Deprecated: `use "table-stats" instead.`,
		Use:        "table-sizes",
		Short:      "Show table sizes of individual tables without their index sizes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return table_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectTableIndexSizesCmd = &cobra.Command{
		Deprecated: `use "table-stats" instead.`,
		Use:        "table-index-sizes",
		Short:      "Show index sizes of individual tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			return table_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectTotalTableSizesCmd = &cobra.Command{
		Deprecated: `use "table-stats" instead.`,
		Use:        "total-table-sizes",
		Short:      "Show total table sizes, including table index sizes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return table_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectUnusedIndexesCmd = &cobra.Command{
		Deprecated: `use "index-stats" instead.`,
		Use:        "unused-indexes",
		Short:      "Show indexes with low usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return index_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectTableRecordCountsCmd = &cobra.Command{
		Deprecated: `use "table-stats" instead.`,
		Use:        "table-record-counts",
		Short:      "Show estimated number of rows per table",
		RunE: func(cmd *cobra.Command, args []string) error {
			return index_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectSeqScansCmd = &cobra.Command{
		Deprecated: `use "index-stats" instead.`,
		Use:        "seq-scans",
		Short:      "Show number of sequential scans recorded against all tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			return index_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectRoleConfigsCmd = &cobra.Command{
		Deprecated: `use "role-stats" instead.`,
		Use:        "role-configs",
		Short:      "Show configuration settings for database roles when they have been modified",
		RunE: func(cmd *cobra.Command, args []string) error {
			return role_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	inspectRoleConnectionsCmd = &cobra.Command{
		Deprecated: `use "role-stats" instead.`,
		Use:        "role-connections",
		Short:      "Show number of active connections for all database roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return role_stats.Run(cmd.Context(), flags.DbConfig, afero.NewOsFs())
		},
	}

	outputDir         string
	inspectDBBranchID string

	reportCmd = &cobra.Command{
		Use:     "report",
		Short:   "Generate a CSV output for all inspect commands",
		PreRunE: preRunVolcengineInspect,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolcengineInspect(cmd, func(ctx context.Context, config pgconn.Config, fsys afero.Fs, options ...func(*pgx.ConnConfig)) error {
				return inspect.Report(ctx, outputDir, config, fsys, options...)
			})
		},
	}
)

type inspectRunner func(context.Context, pgconn.Config, afero.Fs, ...func(*pgx.ConnConfig)) error

func preRunVolcengineInspect(cmd *cobra.Command, args []string) error {
	commandFlags := cmd.Flags()
	direct := commandFlags.Changed("db-url")
	linked := commandFlags.Changed("linked")
	workspace := commandFlags.Changed("workspace-id") || commandFlags.Changed("project-ref")
	branch := commandFlags.Changed("branch-id")
	if direct && (linked || workspace || branch) {
		return errors.New("--db-url cannot be combined with --linked, --workspace-id, --project-ref, or --branch-id")
	}
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	if direct {
		return nil
	}
	return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), "Which project do you want to inspect?")
}

func runVolcengineInspect(cmd *cobra.Command, runner inspectRunner) error {
	config := flags.DbConfig
	if !cmd.Flags().Changed("db-url") {
		fsys := afero.NewOsFs()
		if err := flags.LoadConfig(fsys); err != nil {
			return err
		}
		cfg, err := volcengine.LoadConfigFromEnv()
		if err != nil {
			return err
		}
		config, err = dbconnection.GetVolcengineConfig(cmd.Context(), volcengine.NewClient(cfg), dbconnection.VolcengineParams{
			WorkspaceID: flags.ProjectRef,
			BranchID:    inspectDBBranchID,
		})
		if err != nil {
			return err
		}
	}
	return runner(cmd.Context(), config, afero.NewOsFs())
}

func addVolcengineInspectFlags(cmd *cobra.Command) {
	commandFlags := cmd.Flags()
	commandFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	commandFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	commandFlags.StringVar(&inspectDBBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
}

func isVolcengineInspectCommand(cmd *cobra.Command) bool {
	switch cmd {
	case inspectDBStatsCmd,
		inspectReplicationSlotsCmd,
		inspectLocksCmd,
		inspectBlockingCmd,
		inspectOutliersCmd,
		inspectCallsCmd,
		inspectIndexStatsCmd,
		inspectLongRunningQueriesCmd,
		inspectBloatCmd,
		inspectRoleStatsCmd,
		inspectVacuumStatsCmd,
		inspectTableStatsCmd,
		inspectTrafficProfileCmd,
		reportCmd:
		return true
	default:
		return false
	}
}

func addAllVolcengineInspectFlags() {
	for _, cmd := range []*cobra.Command{
		inspectDBStatsCmd,
		inspectReplicationSlotsCmd,
		inspectLocksCmd,
		inspectBlockingCmd,
		inspectOutliersCmd,
		inspectCallsCmd,
		inspectIndexStatsCmd,
		inspectLongRunningQueriesCmd,
		inspectBloatCmd,
		inspectRoleStatsCmd,
		inspectVacuumStatsCmd,
		inspectTableStatsCmd,
		inspectTrafficProfileCmd,
	} {
		addVolcengineInspectFlags(cmd)
	}
	addVolcengineInspectFlags(reportCmd)
}

func init() {
	inspectFlags := inspectCmd.PersistentFlags()
	inspectFlags.String("db-url", "", "Inspect the database specified by the connection string (must be percent-encoded).")
	inspectFlags.Bool("linked", true, "Use the linked workspace when --db-url is omitted.")
	// Original Supabase local stack mode is not exposed in the Volcengine CLI:
	// inspectFlags.Bool("local", false, "Inspect the local database.")
	inspectCmd.MarkFlagsMutuallyExclusive("db-url", "linked")
	inspectDBCmd.AddCommand(inspectReplicationSlotsCmd)
	inspectDBCmd.AddCommand(inspectIndexStatsCmd)
	inspectDBCmd.AddCommand(inspectLocksCmd)
	inspectDBCmd.AddCommand(inspectBlockingCmd)
	inspectDBCmd.AddCommand(inspectOutliersCmd)
	inspectDBCmd.AddCommand(inspectCallsCmd)
	inspectDBCmd.AddCommand(inspectLongRunningQueriesCmd)
	inspectDBCmd.AddCommand(inspectBloatCmd)
	inspectDBCmd.AddCommand(inspectVacuumStatsCmd)
	addAllVolcengineInspectFlags()
	inspectDBCmd.AddCommand(inspectTableStatsCmd)
	inspectDBCmd.AddCommand(inspectTrafficProfileCmd)
	inspectDBCmd.AddCommand(inspectRoleStatsCmd)
	inspectDBCmd.AddCommand(inspectDBStatsCmd)
	// DEPRECATED
	inspectDBCmd.AddCommand(inspectCacheHitCmd)
	inspectDBCmd.AddCommand(inspectIndexUsageCmd)
	inspectDBCmd.AddCommand(inspectSeqScansCmd)
	inspectDBCmd.AddCommand(inspectUnusedIndexesCmd)
	inspectDBCmd.AddCommand(inspectTotalTableSizesCmd)
	inspectDBCmd.AddCommand(inspectTableIndexSizesCmd)
	inspectDBCmd.AddCommand(inspectTotalIndexSizeCmd)
	inspectDBCmd.AddCommand(inspectIndexSizesCmd)
	inspectDBCmd.AddCommand(inspectTableSizesCmd)
	inspectDBCmd.AddCommand(inspectTableRecordCountsCmd)
	inspectDBCmd.AddCommand(inspectRoleConfigsCmd)
	inspectDBCmd.AddCommand(inspectRoleConnectionsCmd)
	inspectCmd.AddCommand(inspectDBCmd)
	reportCmd.Flags().StringVar(&outputDir, "output-dir", ".", "Path to save CSV files in")
	inspectCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(inspectCmd)
}
