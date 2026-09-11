package solo

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgconn"
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/db/advisors"
	"github.com/volcengine/byted-supabase-cli/internal/db/query"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func soloDBConfig() (pgconn.Config, error) {
	return pgconn.Config{}, fmt.Errorf("direct PostgreSQL inspection is unavailable in Solo Tree; use Supabase data-plane APIs")
}

func newDBCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{GroupID: "data-plane", Use: "db", Short: "Manage Postgres databases"}
	command.AddCommand(
		newDBQueryCommand(ctx),
		newDBAdvisorsCommand(ctx),
	)
	return command
}

func newDBQueryCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var queryFile string
	var output = utils.EnumFlag{Allowed: []string{"json", "table", "csv"}, Value: "json"}
	command := &cobra.Command{
		Use:   "query [sql]",
		Short: "Execute a SQL query against the configured database",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := query.ResolveSQL(args, queryFile, os.Stdin)
			if err != nil {
				return err
			}
			format := output.Value
			if !cmd.Flags().Changed("output") {
				format = "table"
			}
			profile, err := soloconfig.LoadCurrentProfile()
			if err != nil {
				return err
			}
			return query.RunSupabaseHTTP(cmd.Context(), query.SupabaseHTTPParams{
				SupabaseURL:    profile.SupabaseURL,
				ServiceRoleKey: profile.ServiceRoleKey,
				SQL:            sql,
				Format:         format,
				AgentMode:      utils.IsAgentMode(),
			}, ctx.Output)
		},
	}
	command.Flags().StringVarP(&queryFile, "file", "f", "", "Path to a SQL file to execute.")
	command.Flags().VarP(&output, "output", "o", "Output format: table, json, or csv.")
	return command
}

func newDBAdvisorsCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var advisorType = utils.EnumFlag{Allowed: advisors.AllowedTypes, Value: advisors.AllowedTypes[0]}
	var advisorLevel = utils.EnumFlag{Allowed: advisors.AllowedLevels, Value: advisors.AllowedLevels[1]}
	var failOn = utils.EnumFlag{Allowed: append([]string{"none"}, advisors.AllowedLevels...), Value: "none"}
	command := &cobra.Command{
		Use:   "advisors",
		Short: "Check the database for security and performance issues",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			profile, err := soloconfig.LoadCurrentProfile()
			if err != nil {
				return err
			}
			lints, err := advisors.GetExecutorLints(context.Background(), query.SupabaseDataPlaneExecutor{
				SupabaseURL: profile.SupabaseURL, ServiceRoleKey: profile.ServiceRoleKey,
			}, advisorType.Value, advisorLevel.Value)
			if err != nil {
				return err
			}
			return advisors.WriteAndCheck(lints, failOn.Value, os.Stdout)
		},
	}
	command.Flags().Var(&advisorType, "type", "Type of advisors to check: all, security, performance.")
	command.Flags().Var(&advisorLevel, "level", "Minimum issue level to display: info, warn, error.")
	command.Flags().Var(&failOn, "fail-on", "Issue level to exit with non-zero status: none, info, warn, error.")
	_ = ctx
	return command
}
