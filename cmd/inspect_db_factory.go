// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/spf13/cobra"
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
)

func newInspectDBCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "Tools to inspect your Supabase database"}
	cmd.AddCommand(
		newInspectRunnerCommand(ctx, "replication-slots", "Show information about replication slots on the database", replication_slots.Run),
		newInspectRunnerCommand(ctx, "index-stats", "Show combined index size, usage percent, scan counts, and unused status", index_stats.Run),
		newInspectRunnerCommand(ctx, "locks", "Show queries which have taken out an exclusive lock on a relation", locks.Run),
		newInspectRunnerCommand(ctx, "blocking", "Show queries that are holding locks and the queries that are waiting for them to be released", blocking.Run),
		newInspectRunnerCommand(ctx, "outliers", "Show queries from pg_stat_statements ordered by total execution time", outliers.Run),
		newInspectRunnerCommand(ctx, "calls", "Show queries from pg_stat_statements ordered by total times called", calls.Run),
		newInspectRunnerCommand(ctx, "long-running-queries", "Show currently running queries running for longer than 5 minutes", long_running_queries.Run),
		newInspectRunnerCommand(ctx, "bloat", "Estimates space allocated to a relation that is full of dead tuples", bloat.Run),
		newInspectRunnerCommand(ctx, "vacuum-stats", "Show statistics related to vacuum operations per table", vacuum_stats.Run),
		newInspectRunnerCommand(ctx, "table-stats", "Show combined table size, index size, and estimated row count", table_stats.Run),
		newInspectRunnerCommand(ctx, "traffic-profile", "Show read/write activity ratio for tables based on block I/O operations", traffic_profile.Run),
		newInspectRunnerCommand(ctx, "role-stats", "Show information about roles on the database", role_stats.Run),
		newInspectRunnerCommand(ctx, "db-stats", "Show stats such as cache hit rates, total sizes, and WAL size", db_stats.Run),
		newInspectDeprecatedCommand(ctx, "cache-hit", "Show cache hit rates for tables and indices", "use \"db-stats\" instead.", db_stats.Run),
		newInspectDeprecatedCommand(ctx, "index-usage", "Show information about the efficiency of indexes", "use \"index-stats\" instead.", index_stats.Run),
		newInspectDeprecatedCommand(ctx, "seq-scans", "Show number of sequential scans recorded against all tables", "use \"index-stats\" instead.", index_stats.Run),
		newInspectDeprecatedCommand(ctx, "unused-indexes", "Show indexes with low usage", "use \"index-stats\" instead.", index_stats.Run),
		newInspectDeprecatedCommand(ctx, "total-table-sizes", "Show total table sizes, including table index sizes", "use \"table-stats\" instead.", table_stats.Run),
		newInspectDeprecatedCommand(ctx, "table-index-sizes", "Show index sizes of individual tables", "use \"table-stats\" instead.", table_stats.Run),
		newInspectDeprecatedCommand(ctx, "total-index-size", "Show total size of all indexes", "use \"index-stats\" instead.", index_stats.Run),
		newInspectDeprecatedCommand(ctx, "index-sizes", "Show index sizes of individual indexes", "use \"index-stats\" instead.", index_stats.Run),
		newInspectDeprecatedCommand(ctx, "table-sizes", "Show table sizes of individual tables without their index sizes", "use \"table-stats\" instead.", table_stats.Run),
		newInspectDeprecatedCommand(ctx, "table-record-counts", "Show estimated number of rows per table", "use \"table-stats\" instead.", table_stats.Run),
		newInspectDeprecatedCommand(ctx, "role-configs", "Show configuration settings for database roles when they have been modified", "use \"role-stats\" instead.", role_stats.Run),
		newInspectDeprecatedCommand(ctx, "role-connections", "Show number of active connections for all database roles", "use \"role-stats\" instead.", role_stats.Run),
	)
	return cmd
}

func newInspectRunnerCommand(ctx RuntimeContext, use, short string, runner inspectRunnerFactory) *cobra.Command {
	var options inspectOptions
	cmd := &cobra.Command{
		Use: use, Short: short,
		PreRunE: func(cmd *cobra.Command, args []string) error { return preRunInspectFactory(cmd, ctx.FS, &options) },
		RunE:    func(cmd *cobra.Command, args []string) error { return runInspectFactory(cmd, ctx.FS, &options, runner) },
	}
	addInspectTargetFlags(cmd, &options)
	return cmd
}

func newInspectDeprecatedCommand(ctx RuntimeContext, use, short, replacement string, runner inspectRunnerFactory) *cobra.Command {
	cmd := newInspectRunnerCommand(ctx, use, short, runner)
	cmd.Deprecated = replacement
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runner(cmd.Context(), flags.DbConfig, ctx.FS)
	}
	return cmd
}
