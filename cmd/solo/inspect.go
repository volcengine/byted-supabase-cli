package solo

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
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
)

type inspectRunner func(context.Context, pgconn.Config, afero.Fs, ...func(*pgx.ConnConfig)) error

func newInspectCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{GroupID: "data-plane", Use: "inspect", Short: "Inspect the configured Supabase database"}
	db := &cobra.Command{Use: "db", Short: "Inspect database statistics"}
	for _, item := range []struct {
		use, short string
		run        inspectRunner
	}{
		{"replication-slots", "Show replication slots", replication_slots.Run},
		{"index-stats", "Show index statistics", index_stats.Run},
		{"locks", "Show exclusive locks", locks.Run},
		{"blocking", "Show blocking queries", blocking.Run},
		{"outliers", "Show slow query outliers", outliers.Run},
		{"calls", "Show query call statistics", calls.Run},
		{"long-running-queries", "Show long-running queries", long_running_queries.Run},
		{"bloat", "Estimate table bloat", bloat.Run},
		{"vacuum-stats", "Show vacuum statistics", vacuum_stats.Run},
		{"table-stats", "Show table statistics", table_stats.Run},
		{"traffic-profile", "Show table traffic profile", traffic_profile.Run},
		{"role-stats", "Show role statistics", role_stats.Run},
		{"db-stats", "Show database statistics", db_stats.Run},
	} {
		db.AddCommand(newSoloInspectRunner(ctx, item.use, item.short, item.run))
	}
	command.AddCommand(db, newSoloInspectReportCommand(ctx))
	return command
}

func newSoloInspectRunner(ctx commandtree.RuntimeContext, use, short string, run inspectRunner) *cobra.Command {
	return &cobra.Command{
		Use: use, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, err := soloDBConfig()
			if err != nil {
				return err
			}
			return run(cmd.Context(), config, ctx.FS)
		},
	}
}

func newSoloInspectReportCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var outputDir string
	command := &cobra.Command{
		Use: "report", Short: "Generate CSV reports for inspect queries", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, err := soloDBConfig()
			if err != nil {
				return err
			}
			return inspect.Report(cmd.Context(), outputDir, config, ctx.FS)
		},
	}
	command.Flags().StringVar(&outputDir, "output-dir", ".", "Path to save CSV files in")
	return command
}
