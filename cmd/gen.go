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
	"encoding/json"
	"os"
	"strings"
	"time"

	env "github.com/Netflix/go-env"
	"github.com/go-errors/errors"
	"github.com/go-viper/mapstructure/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/gen/bearerjwt"
	"github.com/volcengine/byted-supabase-cli/internal/gen/signingkeys"
	"github.com/volcengine/byted-supabase-cli/internal/gen/types"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/legacy/keys"
	"github.com/volcengine/byted-supabase-cli/pkg/config"
)

var (
	genCmd = &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "gen",
		Short:   "Run code generation tools",
	}

	keyNames keys.CustomName

	genKeysCmd = &cobra.Command{
		Deprecated: `use "gen signing-key" instead.`,
		Use:        "keys",
		Short:      "Generate keys for preview branch",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			es, err := env.EnvironToEnvSet(override)
			if err != nil {
				return err
			}
			if err := env.Unmarshal(es, &keyNames); err != nil {
				return err
			}
			cmd.GroupID = groupManagementAPI
			return cmd.Root().PersistentPreRunE(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format := utils.OutputFormat.Value
			if format == utils.OutputPretty {
				format = utils.OutputEnv
			}
			return keys.Run(cmd.Context(), flags.ProjectRef, format, keyNames, afero.NewOsFs())
		},
	}

	lang = utils.EnumFlag{
		Allowed: []string{
			types.LangTypescript,
			types.LangGo,
			types.LangSwift,
		},
		Value: types.LangTypescript,
	}
	genTypesBranchID   string
	postgrestV9Compat  bool
	swiftAccessControl = utils.EnumFlag{
		Allowed: []string{
			types.SwiftInternalAccessControl,
			types.SwiftPublicAccessControl,
		},
		Value: types.SwiftInternalAccessControl,
	}

	genTypesCmd = &cobra.Command{
		Use:   "types",
		Short: "Generate types from Postgres schema",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			commandFlags := cmd.Flags()
			linked := commandFlags.Changed("linked")
			workspace := commandFlags.Changed("workspace-id") || commandFlags.Changed("project-ref") || commandFlags.Changed("project-id")
			if linked && workspace {
				return errors.New("--linked cannot be combined with --workspace-id, --project-ref, or --project-id")
			}
			// Legacy commands specify language using arg, eg. gen types typescript
			if len(args) > 0 && args[0] != types.LangTypescript && !cmd.Flags().Changed("lang") {
				return errors.New("use --lang flag to specify the typegen language")
			}
			if postgrestV9Compat && lang.Value != types.LangTypescript {
				return errors.New("--postgrest-v9-compat is supported only with --lang typescript")
			}
			if commandFlags.Changed("swift-access-control") && lang.Value != types.LangSwift {
				return errors.New("--swift-access-control is supported only with --lang swift")
			}
			return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), "Which project do you want to generate types for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := flags.LoadConfig(afero.NewOsFs()); err != nil {
				return err
			}
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return types.RunVolcengine(ctx, volcengine.NewClient(cfg), types.VolcengineParams{
				WorkspaceID:        flags.ProjectRef,
				BranchID:           genTypesBranchID,
				Lang:               lang.Value,
				Schemas:            schema,
				PostgrestV9Compat:  postgrestV9Compat,
				SwiftAccessControl: swiftAccessControl.Value,
			}, os.Stdout)
		},
		Example: `  byted-supabase-cli gen types --linked --lang typescript
  byted-supabase-cli gen types --workspace-id workspace-id --lang go --schema public
  byted-supabase-cli gen types --workspace-id workspace-id --lang swift --swift-access-control public`,
	}

	algorithm = utils.EnumFlag{
		Allowed: signingkeys.GetSupportedAlgorithms(),
		Value:   string(config.AlgES256),
	}
	appendKeys bool

	genSigningKeyCmd = &cobra.Command{
		Use:   "signing-key",
		Short: "Generate a JWT signing key",
		Long: `Securely generate a private JWT signing key for use in the CLI or to import in the dashboard.

Supported algorithms:
	ES256 - ECDSA with P-256 curve and SHA-256 (recommended)
	RS256 - RSA with SHA-256
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return signingkeys.Run(cmd.Context(), algorithm.Value, appendKeys, afero.NewOsFs())
		},
	}

	claims   config.CustomClaims
	expiry   time.Time
	validFor time.Duration
	payload  string

	genJWTCmd = &cobra.Command{
		Use:   "bearer-jwt",
		Short: "Generate a Bearer Auth JWT for accessing Data API",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			custom := jwt.MapClaims{}
			if err := parseClaims(custom); err != nil {
				return err
			}
			return bearerjwt.Run(cmd.Context(), custom, os.Stdout, afero.NewOsFs())
		},
	}
)

func init() {
	typeFlags := genTypesCmd.Flags()
	// Volcengine CLI does not support Supabase local stack type generation:
	// typeFlags.Bool("local", false, "Generate types from the local dev database.")
	typeFlags.Bool("linked", false, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	// Original --db-url mode starts a local postgres-meta container; Volcengine
	// type generation uses the deployed branch postgres-meta service instead:
	// typeFlags.String("db-url", "", "Generate types from a database url.")
	typeFlags.StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	typeFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")
	typeFlags.StringVar(&flags.ProjectRef, "project-id", "", "Project ID of the Supabase project. Alias of --workspace-id for Volcengine.")
	typeFlags.StringVar(&genTypesBranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the default branch.")
	markFlagTelemetrySafe(typeFlags.Lookup("project-id"))
	genTypesCmd.MarkFlagsMutuallyExclusive("linked", "workspace-id", "project-ref", "project-id")
	typeFlags.Var(&lang, "lang", "Output language of the generated types.")
	typeFlags.StringSliceVarP(&schema, "schema", "s", []string{}, "Comma separated list of schema to include.")
	typeFlags.Var(&swiftAccessControl, "swift-access-control", "Access control for Swift generated types.")
	typeFlags.BoolVar(&postgrestV9Compat, "postgrest-v9-compat", false, "Generate types compatible with PostgREST v9 and below.")
	// Original query timeout applies only to the local postgres-meta container path:
	// typeFlags.DurationVar(&queryTimeout, "query-timeout", time.Second*15, "Maximum timeout allowed for the database query.")
	genCmd.AddCommand(genTypesCmd)
	keyFlags := genKeysCmd.Flags()
	keyFlags.StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project.")
	markFlagTelemetrySafe(keyFlags.Lookup("project-ref"))
	keyFlags.StringSliceVar(&override, "override-name", []string{}, "Override specific variable names.")
	// Volcengine does not use the deprecated original preview branch key generator:
	// genCmd.AddCommand(genKeysCmd)
	signingKeyFlags := genSigningKeyCmd.Flags()
	signingKeyFlags.Var(&algorithm, "algorithm", "Algorithm for signing key generation.")
	signingKeyFlags.BoolVar(&appendKeys, "append", false, "Append new key to existing keys file instead of overwriting.")
	// Local signing keys are not keys trusted by a remote Volcengine Supabase project:
	// genCmd.AddCommand(genSigningKeyCmd)
	tokenFlags := genJWTCmd.Flags()
	tokenFlags.StringVar(&claims.Role, "role", "", "Postgres role to use.")
	cobra.CheckErr(genJWTCmd.MarkFlagRequired("role"))
	tokenFlags.StringVar(&claims.Subject, "sub", "", "User ID to impersonate.")
	genJWTCmd.Flag("sub").DefValue = "anonymous"
	tokenFlags.TimeVar(&expiry, "exp", time.Time{}, []string{time.RFC3339}, "Expiry timestamp for this token.")
	tokenFlags.DurationVar(&validFor, "valid-for", time.Minute*30, "Validity duration for this token.")
	tokenFlags.StringVar(&payload, "payload", "{}", "Custom claims in JSON format.")
	// Locally signed bearer tokens are not guaranteed to be trusted by a remote project:
	// genCmd.AddCommand(genJWTCmd)
	rootCmd.AddCommand(genCmd)
}

func parseClaims(custom jwt.MapClaims) error {
	// Initialise default claims
	now := time.Now()
	if expiry.IsZero() {
		expiry = now.Add(validFor)
	} else {
		now = expiry.Add(-validFor)
	}
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(expiry)
	// Set is_anonymous = true for authenticated role without explicit user ID
	if strings.EqualFold(claims.Role, "authenticated") && len(claims.Subject) == 0 {
		claims.IsAnon = true
	}
	// Override with custom claims
	if dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Squash:  true,
		Result:  &custom,
	}); err != nil {
		return errors.Errorf("failed to init decoder: %w", err)
	} else if err := dec.Decode(claims); err != nil {
		return errors.Errorf("failed to decode claims: %w", err)
	}
	if err := json.Unmarshal([]byte(payload), &custom); err != nil {
		return errors.Errorf("failed to parse payload: %w", err)
	}
	return nil
}
