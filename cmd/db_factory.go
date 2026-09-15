// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/jackc/pgconn"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/db/advisors"
	dbapiconfig "github.com/volcengine/byted-supabase-cli/internal/db/api_config"
	dbconnection "github.com/volcengine/byted-supabase-cli/internal/db/connection"
	"github.com/volcengine/byted-supabase-cli/internal/db/dump"
	"github.com/volcengine/byted-supabase-cli/internal/db/lint"
	"github.com/volcengine/byted-supabase-cli/internal/db/pull"
	"github.com/volcengine/byted-supabase-cli/internal/db/query"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/pkg/migration"
)

type dbWorkspaceOptions struct {
	WorkspaceID string
	BranchID    string
	DBURL       string
	Linked      bool
}

func newDBCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{GroupID: groupLocalDev, Use: "db", Short: "Manage Postgres Databases"}
	cmd.AddCommand(
		newDBConnectionStringCommand(ctx),
		newDBQueryCommand(ctx),
		newDBAdvisorsCommand(ctx),
		newDBAPIConfigCommand(ctx),
		newDBDumpCommand(ctx),
		newDBPullCommand(ctx),
		newDBLintCommand(ctx),
	)
	return cmd
}

func addDBProjectFlags(cmd *cobra.Command, options *dbWorkspaceOptions) {
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref alias of --workspace-id.")
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project.")
}

func addDBRemoteTargetFlags(cmd *cobra.Command, options *dbWorkspaceOptions, linkedDefault bool) {
	cmd.Flags().StringVar(&options.DBURL, "db-url", "", "Database URL (must be percent-encoded).")
	cmd.Flags().BoolVar(&options.Linked, "linked", linkedDefault, "Use the linked workspace when --db-url is omitted.")
	addDBProjectFlags(cmd, options)
	cmd.MarkFlagsMutuallyExclusive("db-url", "linked")
}

func prepareDBWorkspace(cmd *cobra.Command, fsys afero.Fs, options *dbWorkspaceOptions, prompt string) error {
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if options.DBURL != "" {
		if cmd.Flags().Changed("linked") || workspace || cmd.Flags().Changed("branch-id") {
			return errors.New("--db-url cannot be combined with --linked, --workspace-id, --project-ref, or --branch-id")
		}
		return nil
	}
	if cmd.Flags().Changed("linked") && options.Linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	previous := flags.ProjectRef
	if options.WorkspaceID != "" {
		flags.ProjectRef = options.WorkspaceID
	}
	err := loadOrPromptVolcengineProjectRef(cmd.Context(), fsys, prompt)
	if options.WorkspaceID == "" {
		options.WorkspaceID = flags.ProjectRef
	}
	flags.ProjectRef = previous
	return err
}

func dbConnectionConfig(cmd *cobra.Command, fsys afero.Fs, options dbWorkspaceOptions) (pgconn.Config, error) {
	if options.DBURL != "" {
		return flags.DbConfig, nil
	}
	authContext, err := AuthContextFrom(cmd.Context())
	if err != nil {
		return pgconn.Config{}, err
	}
	return dbconnection.GetVolcengineConfig(cmd.Context(), authContext.VolcengineClient, dbconnection.VolcengineParams{
		WorkspaceID: options.WorkspaceID, BranchID: options.BranchID,
	})
}

func newDBConnectionStringCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	cmd := &cobra.Command{Use: "connection-string", Short: "Retrieve the remote Postgres connection string", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to retrieve a database connection string for?"); err != nil {
				return err
			}
			authContext, err := AuthContextFrom(cmd.Context())
			if err != nil {
				return err
			}
			url, err := dbconnection.GetVolcengineURL(cmd.Context(), authContext.VolcengineClient, dbconnection.VolcengineParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(ctx.Output, url)
			return err
		}}
	addDBProjectFlags(cmd, &options)
	return cmd
}

func newDBQueryCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	var queryFile string
	var output = utils.EnumFlag{Allowed: []string{"json", "table", "csv"}, Value: "json"}
	cmd := &cobra.Command{Use: "query [sql]", Short: "Execute a SQL query against the database", Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to query?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := query.ResolveSQL(args, queryFile, os.Stdin)
			if err != nil {
				return err
			}
			format := output.Value
			if !cmd.Flags().Changed("output") {
				if utils.IsAgentMode() {
					format = "json"
				} else {
					format = "table"
				}
			}
			if options.DBURL != "" {
				return query.RunLocal(cmd.Context(), sql, flags.DbConfig, format, utils.IsAgentMode(), ctx.Output)
			}
			authContext, err := AuthContextFrom(cmd.Context())
			if err != nil {
				return err
			}
			return query.RunVolcengine(cmd.Context(), authContext.VolcengineClient, query.VolcengineParams{WorkspaceID: options.WorkspaceID, BranchID: options.BranchID, SQL: sql, Format: format, AgentMode: utils.IsAgentMode()}, ctx.Output)
		}}
	addDBRemoteTargetFlags(cmd, &options, false)
	cmd.Flags().StringVarP(&queryFile, "file", "f", "", "Path to a SQL file to execute.")
	cmd.Flags().VarP(&output, "output", "o", "Output format: table, json, or csv.")
	return cmd
}

func newDBAdvisorsCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	var advisorType = utils.EnumFlag{Allowed: advisors.AllowedTypes, Value: advisors.AllowedTypes[0]}
	var advisorLevel = utils.EnumFlag{Allowed: advisors.AllowedLevels, Value: advisors.AllowedLevels[1]}
	var failOn = utils.EnumFlag{Allowed: append([]string{"none"}, advisors.AllowedLevels...), Value: "none"}
	cmd := &cobra.Command{Use: "advisors", Short: "Checks database for security and performance issues",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to inspect for advisor issues?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.DBURL != "" {
				return advisors.RunLocal(cmd.Context(), advisorType.Value, advisorLevel.Value, failOn.Value, flags.DbConfig)
			}
			authContext, err := AuthContextFrom(cmd.Context())
			if err != nil {
				return err
			}
			return advisors.RunVolcengine(cmd.Context(), authContext.VolcengineClient, options.WorkspaceID, options.BranchID, advisorType.Value, advisorLevel.Value, failOn.Value)
		}}
	addDBRemoteTargetFlags(cmd, &options, false)
	cmd.Flags().Var(&advisorType, "type", "Type of advisors to check: all, security, performance.")
	cmd.Flags().Var(&advisorLevel, "level", "Minimum issue level to display: info, warn, error.")
	cmd.Flags().Var(&failOn, "fail-on", "Issue level to exit with non-zero status: none, info, warn, error.")
	return cmd
}

func newDBAPIConfigCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "api-config", Short: "Manage DATA API (PostgREST) configuration"}
	cmd.AddCommand(newDBAPIConfigGetCommand(ctx), newDBAPIConfigSetCommand(ctx))
	return cmd
}

func newDBAPIConfigGetCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	var keys []string
	cmd := &cobra.Command{Use: "get", Short: "Get DATA API configuration", PreRunE: func(cmd *cobra.Command, args []string) error {
		return prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to get DATA API config for?")
	},
		RunE: func(cmd *cobra.Command, args []string) error {
			authContext, err := AuthContextFrom(cmd.Context())
			if err != nil {
				return err
			}
			return dbapiconfig.RunVolcengineGet(cmd.Context(), authContext.VolcengineClient, options.WorkspaceID, options.BranchID, keys)
		}}
	addDBProjectFlags(cmd, &options)
	cmd.Flags().BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	cmd.MarkFlagsMutuallyExclusive("linked", "workspace-id", "project-ref")
	cmd.Flags().StringSliceVar(&keys, "key", nil, "Only output specified config key(s). Supports substring match.")
	return cmd
}

func newDBAPIConfigSetCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	var fromJSON string
	cmd := &cobra.Command{Use: "set [KEY=VALUE ...]", Short: "Set DATA API configuration", PreRunE: func(cmd *cobra.Command, args []string) error {
		return prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to set DATA API config for?")
	},
		RunE: func(cmd *cobra.Command, args []string) error {
			authContext, err := AuthContextFrom(cmd.Context())
			if err != nil {
				return err
			}
			return dbapiconfig.RunVolcengineSet(cmd.Context(), authContext.VolcengineClient, options.WorkspaceID, options.BranchID, args, fromJSON, ctx.FS)
		}}
	addDBProjectFlags(cmd, &options)
	cmd.Flags().BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	cmd.MarkFlagsMutuallyExclusive("linked", "workspace-id", "project-ref")
	cmd.Flags().StringVar(&fromJSON, "from-json", "", "Read config patch from a JSON file.")
	return cmd
}

func newDBDumpCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	var file string
	var dataOnly, useCopy, roleOnly, keepComments, dryRun bool
	var schema, exclude []string
	cmd := &cobra.Command{Use: "dump", Short: "Dumps data or schemas from the remote database", PreRunE: func(cmd *cobra.Command, args []string) error {
		return prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to dump?")
	},
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := dbConnectionConfig(cmd, ctx.FS, options)
			if err != nil {
				return err
			}
			opts := []migration.DumpOptionFunc{migration.WithSchema(schema...), migration.WithoutTable(exclude...), migration.WithComments(keepComments), migration.WithColumnInsert(!useCopy)}
			return dump.Run(cmd.Context(), file, config, dataOnly, roleOnly, dryRun, ctx.FS, opts...)
		}}
	addDBRemoteTargetFlags(cmd, &options, true)
	f := cmd.Flags()
	f.StringVarP(&file, "file", "f", "", "File path to save the dumped contents.")
	f.BoolVar(&dataOnly, "data-only", false, "Dumps only data records.")
	f.BoolVar(&useCopy, "use-copy", false, "Use copy statements in place of inserts.")
	f.BoolVar(&roleOnly, "role-only", false, "Dumps only cluster roles.")
	f.BoolVar(&keepComments, "keep-comments", false, "Keeps commented lines in the dump.")
	f.BoolVar(&dryRun, "dry-run", false, "Prints the pg_dump script that would be executed.")
	f.StringSliceVarP(&schema, "schema", "s", nil, "Comma separated list of schema to include.")
	f.StringSliceVarP(&exclude, "exclude", "x", nil, "List of schema.tables to exclude from data-only dump.")
	cmd.MarkFlagsMutuallyExclusive("role-only", "data-only")
	cmd.MarkFlagsMutuallyExclusive("keep-comments", "data-only")
	cmd.MarkFlagsMutuallyExclusive("schema", "role-only")
	return cmd
}

func newDBPullCommand(ctx RuntimeContext) *cobra.Command {
	var options dbWorkspaceOptions
	var schema []string
	cmd := &cobra.Command{Use: "pull [migration name]", Short: "Pull schema from the remote database", PreRunE: func(cmd *cobra.Command, args []string) error {
		return prepareDBWorkspace(cmd, ctx.FS, &options, "Which project do you want to pull schema from?")
	}, RunE: func(cmd *cobra.Command, args []string) error {
		config, err := dbConnectionConfig(cmd, ctx.FS, options)
		if err != nil {
			return err
		}
		name := "remote_schema"
		if len(args) > 0 {
			name = args[0]
		}
		return pull.RunSchemaSnapshot(cmd.Context(), schema, config, name, ctx.FS)
	}}
	addDBRemoteTargetFlags(cmd, &options, true)
	cmd.Flags().StringSliceVarP(&schema, "schema", "s", nil, "Comma separated list of schema to include.")
	return cmd
}

func newDBLintCommand(ctx RuntimeContext) *cobra.Command {
	var schema []string
	var level = utils.EnumFlag{Allowed: lint.AllowedLevels, Value: lint.AllowedLevels[0]}
	var fail = utils.EnumFlag{Allowed: append([]string{"none"}, lint.AllowedLevels...), Value: "none"}
	cmd := &cobra.Command{Use: "lint", Short: "Checks local database for typing error", RunE: func(cmd *cobra.Command, args []string) error {
		return lint.Run(cmd.Context(), schema, level.Value, fail.Value, flags.DbConfig, ctx.FS)
	}}
	SetAuthPolicy(cmd, AuthPolicyNone)
	cmd.Flags().String("db-url", "", "Lints the database specified by the connection string (must be percent-encoded).")
	cmd.Flags().StringSliceVarP(&schema, "schema", "s", nil, "Comma separated list of schema to include.")
	cmd.Flags().Var(&level, "level", "Error level to emit.")
	cmd.Flags().Var(&fail, "fail-on", "Error level to exit with non-zero status.")
	return cmd
}
