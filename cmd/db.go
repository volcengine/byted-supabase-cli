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
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/db/advisors"
	dbapiconfig "github.com/volcengine/byted-supabase-cli/internal/db/api_config"
	dbconnection "github.com/volcengine/byted-supabase-cli/internal/db/connection"
	"github.com/volcengine/byted-supabase-cli/internal/db/diff"
	"github.com/volcengine/byted-supabase-cli/internal/db/dump"
	"github.com/volcengine/byted-supabase-cli/internal/db/lint"
	"github.com/volcengine/byted-supabase-cli/internal/db/pull"
	"github.com/volcengine/byted-supabase-cli/internal/db/push"
	"github.com/volcengine/byted-supabase-cli/internal/db/query"
	"github.com/volcengine/byted-supabase-cli/internal/db/reset"
	"github.com/volcengine/byted-supabase-cli/internal/db/start"
	"github.com/volcengine/byted-supabase-cli/internal/db/test"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/legacy/branch/create"
	"github.com/volcengine/byted-supabase-cli/legacy/branch/delete"
	"github.com/volcengine/byted-supabase-cli/legacy/branch/list"
	"github.com/volcengine/byted-supabase-cli/legacy/branch/switch_"
	"github.com/volcengine/byted-supabase-cli/pkg/migration"
)

var (
	dbCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "db",
		Short:   "Manage Postgres databases",
	}

	dbBranchCmd = &cobra.Command{
		Hidden: true,
		Use:    "branch",
		Short:  "Manage local database branches",
		Long:   "Manage local database branches. Each branch is associated with a separate local database. Forking remote databases is NOT supported.",
	}

	dbBranchCreateCmd = &cobra.Command{
		Deprecated: "use \"branches create <name>\" instead.\n",
		Use:        "create <branch name>",
		Short:      "Create a branch",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return create.Run(args[0], afero.NewOsFs())
		},
	}

	dbBranchDeleteCmd = &cobra.Command{
		Deprecated: "use \"branches delete <branch-id>\" instead.\n",
		Use:        "delete <branch name>",
		Short:      "Delete a branch",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return delete.Run(args[0], afero.NewOsFs())
		},
	}

	dbBranchListCmd = &cobra.Command{
		Deprecated: "use \"branches list\" instead.\n",
		Use:        "list",
		Short:      "List branches",
		RunE: func(cmd *cobra.Command, args []string) error {
			return list.Run(afero.NewOsFs(), os.Stdout)
		},
	}

	dbSwitchCmd = &cobra.Command{
		Deprecated: "use \"branches create <name>\" instead.\n",
		Use:        "switch <branch name>",
		Short:      "Switch the active branch",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return switch_.Run(cmd.Context(), args[0], afero.NewOsFs())
		},
	}

	useMigra       bool
	usePgAdmin     bool
	usePgSchema    bool
	usePgDelta     bool
	pullDiffEngine = utils.EnumFlag{
		Allowed: []string{"migra", "pg-delta"},
		Value:   "migra",
	}
	diffFrom   string
	diffTo     string
	outputPath string
	schema     []string
	file       string

	dbDiffCmd = &cobra.Command{
		Use:   "diff",
		Short: "Diffs the local database for schema changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(diffFrom) > 0 || len(diffTo) > 0 {
				switch {
				case len(diffFrom) == 0 || len(diffTo) == 0:
					return fmt.Errorf("must set both --from and --to when using explicit diff mode")
				default:
					return diff.RunExplicit(cmd.Context(), diffFrom, diffTo, schema, outputPath, afero.NewOsFs())
				}
			}
			useDelta := shouldUsePgDelta()
			if usePgAdmin {
				return diff.RunPgAdmin(cmd.Context(), schema, file, flags.DbConfig, afero.NewOsFs())
			}
			differ := diff.DiffSchemaMigra
			if usePgSchema {
				differ = diff.DiffPgSchema
				fmt.Fprintln(os.Stderr, utils.Yellow("WARNING:"), "--use-pg-schema flag is experimental and may not include all entities, such as views and grants.")
			} else if useDelta {
				differ = diff.DiffPgDelta
			}
			return diff.Run(cmd.Context(), schema, file, flags.DbConfig, differ, useDelta, afero.NewOsFs())
		},
	}

	dataOnly     bool
	useCopy      bool
	roleOnly     bool
	keepComments bool
	excludeTable []string

	dbDumpCmd = &cobra.Command{
		Use:   "dump",
		Short: "Dumps data or schemas from the remote database",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if useCopy || len(excludeTable) > 0 {
				if err := cmd.MarkFlagRequired("data-only"); err != nil {
					return err
				}
			}
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
			return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), "Which project do you want to dump?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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
					BranchID:    dbDumpBranchID,
				})
				if err != nil {
					return err
				}
			}
			opts := []migration.DumpOptionFunc{
				migration.WithSchema(schema...),
				migration.WithoutTable(excludeTable...),
				migration.WithComments(keepComments),
				migration.WithColumnInsert(!useCopy),
			}
			return dump.Run(cmd.Context(), file, config, dataOnly, roleOnly, dryRun, afero.NewOsFs(), opts...)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			if len(file) > 0 && !dryRun {
				if absPath, err := filepath.Abs(file); err != nil {
					fmt.Fprintln(os.Stderr, "Dumped schema to "+utils.Bold(file)+".")
				} else {
					fmt.Fprintln(os.Stderr, "Dumped schema to "+utils.Bold(absPath)+".")
				}
			}
		},
	}

	dryRun       bool
	includeAll   bool
	includeRoles bool
	includeSeed  bool

	dbPushCmd = &cobra.Command{
		Use:   "push",
		Short: "Push new migrations to the remote database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return push.Run(cmd.Context(), dryRun, includeAll, includeRoles, includeSeed, flags.DbConfig, afero.NewOsFs())
		},
	}

	dbPullCmd = &cobra.Command{
		Use:   "pull [migration name]",
		Short: "Pull schema from the remote database",
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
			return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), "Which project do you want to pull schema from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "remote_schema"
			if len(args) > 0 {
				name = args[0]
			}
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
					BranchID:    dbPullBranchID,
				})
				if err != nil {
					return err
				}
			}
			return pull.RunSchemaSnapshot(cmd.Context(), schema, config, name, afero.NewOsFs())
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			fmt.Println("Finished " + utils.Aqua("byted-supabase-cli db pull") + ".")
		},
	}

	dbRemoteCmd = &cobra.Command{
		Hidden: true,
		Use:    "remote",
		Short:  "Manage remote databases",
	}

	dbRemoteChangesCmd = &cobra.Command{
		Deprecated: "use \"db diff --use-migra --linked\" instead.\n",
		Use:        "changes",
		Short:      "Show changes on the remote database",
		Long:       "Show changes on the remote database since last migration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return diff.Run(cmd.Context(), schema, file, flags.DbConfig, diff.DiffSchemaMigra, false, afero.NewOsFs())
		},
	}

	dbRemoteCommitCmd = &cobra.Command{
		Deprecated: "use \"db pull\" instead.\n",
		Use:        "commit",
		Short:      "Commit remote changes as a new migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			useDelta := shouldUsePgDelta()
			return pull.Run(cmd.Context(), schema, flags.DbConfig, "remote_commit", useDelta, false, diff.DiffSchemaMigra, afero.NewOsFs())
		},
	}

	noSeed      bool
	lastVersion uint

	dbResetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Resets the local database to current migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if noSeed {
				utils.Config.Db.Seed.Enabled = false
			}
			return reset.Run(cmd.Context(), migrationVersion, lastVersion, flags.DbConfig, afero.NewOsFs())
		},
	}

	level = utils.EnumFlag{
		Allowed: lint.AllowedLevels,
		Value:   lint.AllowedLevels[0],
	}

	lintFailOn = utils.EnumFlag{
		Allowed: append([]string{"none"}, lint.AllowedLevels...),
		Value:   "none",
	}

	dbLintCmd = &cobra.Command{
		Use:   "lint",
		Short: "Checks local database for typing error",
		RunE: func(cmd *cobra.Command, args []string) error {
			return lint.Run(cmd.Context(), schema, level.Value, lintFailOn.Value, flags.DbConfig, afero.NewOsFs())
		},
	}

	fromBackup string

	dbStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Starts local Postgres database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return start.Run(cmd.Context(), fromBackup, afero.NewOsFs())
		},
	}

	dbConnectionStringBranchID string
	dbDumpBranchID             string
	dbPullBranchID             string
	dbQueryBranchID            string
	dbAdvisorsBranchID         string

	dbConnectionStringCmd = &cobra.Command{
		Use:   "connection-string",
		Short: "Retrieve the remote Postgres connection string",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			fsys := afero.NewOsFs()
			if err := loadOrPromptVolcengineProjectRef(ctx, fsys, "Which project do you want to retrieve a database connection string for?"); err != nil {
				return err
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			connectionURL, err := dbconnection.GetVolcengineURL(ctx, volcengine.NewClient(cfg), dbconnection.VolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    dbConnectionStringBranchID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, connectionURL)
			return nil
		},
	}

	dbTestCmd = &cobra.Command{
		Hidden: true,
		Use:    "test [path] ...",
		Short:  "Tests local database with pgTAP",
		RunE: func(cmd *cobra.Command, args []string) error {
			return test.Run(cmd.Context(), args, flags.DbConfig, afero.NewOsFs())
		},
	}

	queryFile   string
	queryOutput = utils.EnumFlag{
		Allowed: []string{"json", "table", "csv"},
		Value:   "json",
	}

	dbQueryCmd = &cobra.Command{
		Use:   "query [sql]",
		Short: "Execute a SQL query against the database",
		Long: `Execute a SQL query against a linked Volcengine Supabase project or an explicit Postgres URL.

When used by an AI coding agent (auto-detected or via --agent=yes), the default
output format is JSON with an untrusted data warning envelope. When used by a
human (--agent=no or no agent detected), the default output format is table
without the envelope.`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
			return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), "Which project do you want to query?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sql, err := query.ResolveSQL(args, queryFile, os.Stdin)
			if err != nil {
				return err
			}
			agentMode := utils.IsAgentMode()
			// If user didn't explicitly set --output, pick default based on agent mode
			outputFormat := queryOutput.Value
			if outputFlag := cmd.Flags().Lookup("output"); outputFlag != nil && !outputFlag.Changed {
				if agentMode {
					outputFormat = "json"
				} else {
					outputFormat = "table"
				}
			}
			if cmd.Flags().Changed("db-url") {
				return query.RunLocal(cmd.Context(), sql, flags.DbConfig, outputFormat, agentMode, os.Stdout)
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return query.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), query.VolcengineParams{
				WorkspaceID: flags.ProjectRef,
				BranchID:    dbQueryBranchID,
				SQL:         sql,
				Format:      outputFormat,
				AgentMode:   agentMode,
			}, os.Stdout)
		},
	}

	advisorType = utils.EnumFlag{
		Allowed: advisors.AllowedTypes,
		Value:   advisors.AllowedTypes[0],
	}

	advisorLevel = utils.EnumFlag{
		Allowed: advisors.AllowedLevels,
		Value:   advisors.AllowedLevels[1],
	}

	advisorFailOn = utils.EnumFlag{
		Allowed: append([]string{"none"}, advisors.AllowedLevels...),
		Value:   "none",
	}

	dbAdvisorsCmd = &cobra.Command{
		Use:   "advisors",
		Short: "Checks database for security and performance issues",
		Long:  "Inspects the database for common security and performance issues such as missing RLS policies, unindexed foreign keys, exposed auth.users, and more.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
			return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), "Which project do you want to inspect for advisor issues?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("db-url") {
				return advisors.RunLocal(cmd.Context(), advisorType.Value, advisorLevel.Value, advisorFailOn.Value, flags.DbConfig)
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return advisors.RunVolcengine(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, dbAdvisorsBranchID, advisorType.Value, advisorLevel.Value, advisorFailOn.Value)
		},
	}
)

// --- db api-config ---

var (
	dbAPIConfigCmd = &cobra.Command{
		Use:   "api-config",
		Short: "Manage DATA API (PostgREST) configuration",
	}

	dbAPIConfigGetKeys   []string
	dbAPIConfigGetBranch string

	dbAPIConfigGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get DATA API configuration",
		Long:  "Get DATA API (PostgREST) configuration for the linked project. Returns all config keys by default, or filter with --key.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineDB(cmd, "Which project do you want to get DATA API config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return dbapiconfig.RunVolcengineGet(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, dbAPIConfigGetBranch, dbAPIConfigGetKeys)
		},
	}

	dbAPIConfigSetBranch string
	dbAPIConfigSetJSON   string

	dbAPIConfigSetCmd = &cobra.Command{
		Use:   "set [KEY=VALUE ...]",
		Short: "Set DATA API configuration",
		Long: `Set DATA API (PostgREST) configuration for the linked project.

Provide one or more KEY=VALUE pairs separated by spaces:
  db api-config set db_schema=public,storage max_rows=500

Or read from a JSON file:
  db api-config set --from-json config.json

Unspecified fields retain their current values (CLI auto-merges with existing config).

Available keys:
  db_schema              Comma-separated list of schemas exposed via REST API
  db_extra_search_path   Comma-separated list of extra search path schemas
  max_rows               Maximum number of rows returned per request (0-1000000)`,
		Example: `  supabase db api-config set db_schema=public,storage
  supabase db api-config set db_schema=public,storage db_extra_search_path=public,extensions max_rows=1000
  supabase db api-config set --from-json config.json`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineDB(cmd, "Which project do you want to set DATA API config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return dbapiconfig.RunVolcengineSet(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, dbAPIConfigSetBranch, args, dbAPIConfigSetJSON, afero.NewOsFs())
		},
	}
)

func preRunVolcengineDB(cmd *cobra.Command, prompt string) error {
	linked := cmd.Flags().Changed("linked")
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
}

func shouldUsePgDelta() bool {
	return utils.IsPgDeltaEnabled() || usePgDelta || viper.GetBool("EXPERIMENTAL_PG_DELTA")
}

func init() {
	// Build branch command
	dbBranchCmd.AddCommand(dbBranchCreateCmd)
	dbBranchCmd.AddCommand(dbBranchDeleteCmd)
	dbBranchCmd.AddCommand(dbBranchListCmd)
	dbBranchCmd.AddCommand(dbSwitchCmd)
	// Volcengine CLI exposes remote branch management through the top-level
	// branches command. The legacy local database branch command is not
	// registered to avoid hidden local-stack workflows.
	// dbCmd.AddCommand(dbBranchCmd)
	// Build diff command
	diffFlags := dbDiffCmd.Flags()
	diffFlags.BoolVar(&useMigra, "use-migra", true, "Use migra to generate schema diff.")
	diffFlags.BoolVar(&usePgAdmin, "use-pgadmin", false, "Use pgAdmin to generate schema diff.")
	diffFlags.BoolVar(&usePgSchema, "use-pg-schema", false, "Use pg-schema-diff to generate schema diff.")
	diffFlags.BoolVar(&usePgDelta, "use-pg-delta", false, "Use pg-delta to generate schema diff.")
	dbDiffCmd.MarkFlagsMutuallyExclusive("use-migra", "use-pgadmin", "use-pg-schema", "use-pg-delta")
	diffFlags.StringVar(&diffFrom, "from", "", "Diff from local, linked, migrations, or a Postgres URL.")
	diffFlags.StringVar(&diffTo, "to", "", "Diff to local, linked, migrations, or a Postgres URL.")
	diffFlags.StringVarP(&outputPath, "output", "o", "", "Write explicit diff output to a file path.")
	diffFlags.String("db-url", "", "Diffs against the database specified by the connection string (must be percent-encoded).")
	diffFlags.Bool("linked", false, "Diffs local migration files against the linked project.")
	diffFlags.Bool("local", true, "Diffs local migration files against the local database.")
	dbDiffCmd.MarkFlagsMutuallyExclusive("db-url", "linked", "local")
	diffFlags.StringVarP(&file, "file", "f", "", "Saves schema diff to a new migration file.")
	diffFlags.StringSliceVarP(&schema, "schema", "s", []string{}, "Comma separated list of schema to include.")
	// Volcengine does not expose local stack/shadow database workflows at this stage:
	// dbCmd.AddCommand(dbDiffCmd)
	// Build dump command
	dumpFlags := dbDumpCmd.Flags()
	dumpFlags.BoolVar(&dryRun, "dry-run", false, "Prints the pg_dump script that would be executed.")
	dumpFlags.BoolVar(&dataOnly, "data-only", false, "Dumps only data records.")
	dumpFlags.BoolVar(&useCopy, "use-copy", false, "Use copy statements in place of inserts.")
	dumpFlags.StringSliceVarP(&excludeTable, "exclude", "x", []string{}, "List of schema.tables to exclude from data-only dump.")
	dumpFlags.BoolVar(&roleOnly, "role-only", false, "Dumps only cluster roles.")
	dbDumpCmd.MarkFlagsMutuallyExclusive("role-only", "data-only")
	dumpFlags.BoolVar(&keepComments, "keep-comments", false, "Keeps commented lines from pg_dump output.")
	dbDumpCmd.MarkFlagsMutuallyExclusive("keep-comments", "data-only")
	dumpFlags.StringVarP(&file, "file", "f", "", "File path to save the dumped contents.")
	dumpFlags.String("db-url", "", "Dumps from the database specified by the connection string (must be percent-encoded).")
	dumpFlags.Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	// Volcengine CLI does not expose local stack workflows:
	// dumpFlags.Bool("local", false, "Dumps from the local database.")
	dbDumpCmd.MarkFlagsMutuallyExclusive("db-url", "linked")
	dumpFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	dumpFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	dumpFlags.StringVar(&dbDumpBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	// Volcengine resolves the managed Postgres password on demand via
	// DescribeDBAccountConnection. Exposing --password here is misleading and
	// encourages secrets in shell history, so the original flag stays disabled.
	// dumpFlags.StringVarP(&dbPassword, "password", "p", "", "Password to your remote Postgres database.")
	// cobra.CheckErr(viper.BindPFlag("DB_PASSWORD", dumpFlags.Lookup("password")))
	dumpFlags.StringSliceVarP(&schema, "schema", "s", []string{}, "Comma separated list of schema to include.")
	dbDumpCmd.MarkFlagsMutuallyExclusive("schema", "role-only")
	dbCmd.AddCommand(dbDumpCmd)
	// Build push command
	pushFlags := dbPushCmd.Flags()
	pushFlags.BoolVar(&includeAll, "include-all", false, "Include all migrations not found on remote history table.")
	pushFlags.BoolVar(&includeRoles, "include-roles", false, "Include custom roles from "+utils.CustomRolesPath+".")
	pushFlags.BoolVar(&includeSeed, "include-seed", false, "Include seed data from your config.")
	pushFlags.BoolVar(&dryRun, "dry-run", false, "Print the migrations that would be applied, but don't actually apply them.")
	pushFlags.String("db-url", "", "Pushes to the database specified by the connection string (must be percent-encoded).")
	pushFlags.Bool("linked", true, "Pushes to the linked project.")
	pushFlags.Bool("local", false, "Pushes to the local database.")
	dbPushCmd.MarkFlagsMutuallyExclusive("db-url", "linked", "local")
	pushFlags.StringVarP(&dbPassword, "password", "p", "", "Password to your remote Postgres database.")
	cobra.CheckErr(viper.BindPFlag("DB_PASSWORD", pushFlags.Lookup("password")))
	// Volcengine does not support remote migration execution/history at this stage:
	// dbCmd.AddCommand(dbPushCmd)
	// Build pull command
	pullFlags := dbPullCmd.Flags()
	// Volcengine pull currently creates a local schema snapshot only. Original
	// migration-history/declarative options can be restored once remote
	// migration history is supported:
	// pullFlags.BoolVar(&usePgDelta, "use-pg-delta", false, "Use pg-delta to pull declarative schema.")
	// pullFlags.Var(&pullDiffEngine, "diff-engine", "Diff engine to use for migration-style db pull.")
	pullFlags.StringSliceVarP(&schema, "schema", "s", []string{}, "Comma separated list of schema to include.")
	pullFlags.String("db-url", "", "Pulls from the database specified by the connection string (must be percent-encoded).")
	pullFlags.Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	// Original Supabase local stack mode is not exposed in the Volcengine CLI:
	// pullFlags.Bool("local", false, "Pulls from the local database.")
	pullFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	pullFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	pullFlags.StringVar(&dbPullBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	// Volcengine resolves the managed Postgres password on demand via
	// DescribeDBAccountConnection. Exposing --password here is misleading and
	// encourages secrets in shell history, so the original flag stays disabled.
	// pullFlags.StringVarP(&dbPassword, "password", "p", "", "Password to your remote Postgres database.")
	// cobra.CheckErr(viper.BindPFlag("DB_PASSWORD", pullFlags.Lookup("password")))
	dbCmd.AddCommand(dbPullCmd)
	// Build remote command
	remoteFlags := dbRemoteCmd.PersistentFlags()
	remoteFlags.StringSliceVarP(&schema, "schema", "s", []string{}, "Comma separated list of schema to include.")
	remoteFlags.String("db-url", "", "Connect using the specified Postgres URL (must be percent-encoded).")
	remoteFlags.Bool("linked", true, "Connect to the linked project.")
	dbRemoteCmd.MarkFlagsMutuallyExclusive("db-url", "linked")
	remoteFlags.StringVarP(&dbPassword, "password", "p", "", "Password to your remote Postgres database.")
	cobra.CheckErr(viper.BindPFlag("DB_PASSWORD", remoteFlags.Lookup("password")))
	dbRemoteCmd.AddCommand(dbRemoteChangesCmd)
	dbRemoteCmd.AddCommand(dbRemoteCommitCmd)
	// Legacy remote migration helpers rely on unsupported migration history and
	// local-stack workflows in Volcengine CLI.
	// dbCmd.AddCommand(dbRemoteCmd)
	// Build reset command
	resetFlags := dbResetCmd.Flags()
	resetFlags.String("db-url", "", "Resets the database specified by the connection string (must be percent-encoded).")
	resetFlags.Bool("linked", false, "Resets the linked project with local migrations.")
	resetFlags.Bool("local", true, "Resets the local database with local migrations.")
	resetFlags.BoolVar(&noSeed, "no-seed", false, "Skip running the seed script after reset.")
	dbResetCmd.MarkFlagsMutuallyExclusive("db-url", "linked", "local")
	resetFlags.StringVar(&migrationVersion, "version", "", "Reset up to the specified version.")
	resetFlags.UintVar(&lastVersion, "last", 0, "Reset up to the last n migration versions.")
	dbResetCmd.MarkFlagsMutuallyExclusive("version", "last")
	// Volcengine does not expose local stack or remote migration reset workflows at this stage:
	// dbCmd.AddCommand(dbResetCmd)
	// Build lint command
	lintFlags := dbLintCmd.Flags()
	lintFlags.String("db-url", "", "Lints the database specified by the connection string (must be percent-encoded).")
	lintFlags.Bool("linked", false, "Lints the linked project for schema errors.")
	lintFlags.Bool("local", true, "Lints the local database for schema errors.")
	dbLintCmd.MarkFlagsMutuallyExclusive("db-url", "linked", "local")
	lintFlags.StringSliceVarP(&schema, "schema", "s", []string{}, "Comma separated list of schema to include.")
	lintFlags.Var(&level, "level", "Error level to emit.")
	lintFlags.Var(&lintFailOn, "fail-on", "Error level to exit with non-zero status.")
	// Volcengine Supabase PostgreSQL does not provide the plpgsql_check
	// extension required by the original db lint behavior:
	// dbCmd.AddCommand(dbLintCmd)
	// Build start command
	startFlags := dbStartCmd.Flags()
	startFlags.StringVar(&fromBackup, "from-backup", "", "Path to a logical backup file.")
	// Volcengine does not expose local stack startup workflows at this stage:
	// dbCmd.AddCommand(dbStartCmd)
	connectionStringFlags := dbConnectionStringCmd.Flags()
	connectionStringFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	connectionStringFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	connectionStringFlags.StringVar(&dbConnectionStringBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	dbCmd.AddCommand(dbConnectionStringCmd)
	// Build test command
	// db test depends on local pgTAP/local-stack semantics and is not registered
	// for Volcengine CLI.
	// dbCmd.AddCommand(dbTestCmd)
	testFlags := dbTestCmd.Flags()
	testFlags.String("db-url", "", "Tests the database specified by the connection string (must be percent-encoded).")
	testFlags.Bool("linked", false, "Runs pgTAP tests on the linked project.")
	testFlags.Bool("local", true, "Runs pgTAP tests on the local database.")
	dbTestCmd.MarkFlagsMutuallyExclusive("db-url", "linked", "local")
	// Build query command
	queryFlags := dbQueryCmd.Flags()
	queryFlags.String("db-url", "", "Queries the database specified by the connection string (must be percent-encoded).")
	queryFlags.Bool("linked", false, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	queryFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	queryFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	queryFlags.StringVar(&dbQueryBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	// Original Supabase local stack mode is not exposed in the Volcengine CLI:
	// queryFlags.Bool("local", true, "Queries the local database.")
	queryFlags.StringVarP(&queryFile, "file", "f", "", "Path to a SQL file to execute.")
	queryFlags.VarP(&queryOutput, "output", "o", "Output format: table, json, or csv.")
	dbCmd.AddCommand(dbQueryCmd)
	// Build advisors command
	advisorsFlags := dbAdvisorsCmd.Flags()
	advisorsFlags.String("db-url", "", "Checks the database specified by the connection string (must be percent-encoded).")
	advisorsFlags.Bool("linked", false, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	advisorsFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	advisorsFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	advisorsFlags.StringVar(&dbAdvisorsBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	// Original Supabase local stack mode is not exposed in the Volcengine CLI:
	// advisorsFlags.Bool("local", true, "Checks the local database for issues.")
	advisorsFlags.Var(&advisorType, "type", "Type of advisors to check: all, security, performance.")
	advisorsFlags.Var(&advisorLevel, "level", "Minimum issue level to display: info, warn, error.")
	advisorsFlags.Var(&advisorFailOn, "fail-on", "Issue level to exit with non-zero status: none, info, warn, error.")
	dbCmd.AddCommand(dbAdvisorsCmd)
	// Build api-config command
	dbAPIConfigGetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	dbAPIConfigGetCmd.Flags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	dbAPIConfigGetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	dbAPIConfigGetCmd.Flags().StringVar(&dbAPIConfigGetBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	dbAPIConfigGetCmd.Flags().StringSliceVar(&dbAPIConfigGetKeys, "key", nil, "Only output specified config key(s). Supports substring match.")
	dbAPIConfigSetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	dbAPIConfigSetCmd.Flags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	dbAPIConfigSetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	dbAPIConfigSetCmd.Flags().StringVar(&dbAPIConfigSetBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	dbAPIConfigSetCmd.Flags().StringVar(&dbAPIConfigSetJSON, "from-json", "", "Read config patch from a JSON file.")
	dbAPIConfigCmd.AddCommand(dbAPIConfigGetCmd)
	dbAPIConfigCmd.AddCommand(dbAPIConfigSetCmd)
	dbCmd.AddCommand(dbAPIConfigCmd)
	rootCmd.AddCommand(dbCmd)
}
