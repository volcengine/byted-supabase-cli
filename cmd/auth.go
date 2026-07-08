// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	authconfig "github.com/volcengine/byted-supabase-cli/internal/auth/config"
	authhooks "github.com/volcengine/byted-supabase-cli/internal/auth/hooks"
	authtp "github.com/volcengine/byted-supabase-cli/internal/auth/thirdparty"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	authCmd = &cobra.Command{
		GroupID: groupManagementAPI,
		Use:     "auth",
		Short:   "Manage Supabase Auth configuration",
	}

	// --- config ---

	authConfigCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage Auth general configuration",
	}

	authConfigGetKeys   []string
	authConfigGetBranch string

	authConfigGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get Auth configuration",
		Long:  "Get Auth configuration for the linked project. Returns all config keys by default, or filter with --key.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to get auth config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authconfig.RunVolcengineGet(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authConfigGetBranch, authConfigGetKeys)
		},
	}

	authConfigSetBranch string
	authConfigSetJSON   string

	authConfigSetCmd = &cobra.Command{
		Use:   "set [KEY=VALUE ...]",
		Short: "Set Auth configuration",
		Long:  "Set Auth configuration for the linked project. Provide KEY=VALUE pairs or --from-json <file>.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to set auth config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authconfig.RunVolcengineSet(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authConfigSetBranch, args, authConfigSetJSON, afero.NewOsFs())
		},
	}

	// --- hooks ---

	authHooksCmd = &cobra.Command{
		Use:   "hooks",
		Short: "Manage Auth Hooks configuration",
	}

	authHooksGetKeys   []string
	authHooksGetBranch string

	authHooksGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get Auth Hooks configuration",
		Long:  "Get Auth Hooks configuration for the linked project. Returns all hooks config keys by default, or filter with --key.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to get hooks config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authhooks.RunVolcengineGet(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authHooksGetBranch, authHooksGetKeys)
		},
	}

	authHooksSetBranch string
	authHooksSetJSON   string

	authHooksSetCmd = &cobra.Command{
		Use:   "set [KEY=VALUE ...]",
		Short: "Set Auth Hooks configuration",
		Long:  "Set Auth Hooks configuration for the linked project. Provide KEY=VALUE pairs or --from-json <file>.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to set hooks config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authhooks.RunVolcengineSet(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authHooksSetBranch, args, authHooksSetJSON, afero.NewOsFs())
		},
	}

	// --- third-party ---

	authThirdPartyCmd = &cobra.Command{
		Use:     "third-party",
		Aliases: []string{"tp"},
		Short:   "Manage Third-Party Auth providers",
	}

	authTPListBranch string

	authTPListCmd = &cobra.Command{
		Use:   "list",
		Short: "List Third-Party Auth providers",
		Long:  "List all configured Third-Party Auth providers for the linked project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to list third-party providers for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authtp.RunVolcengineList(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authTPListBranch)
		},
	}

	authTPAddBranch        string
	authTPAddOIDCIssuerURL string
	authTPAddJWKSKid       string
	authTPAddJWKSAlgorithm string
	authTPAddJWKSPublicKey string

	authTPAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add a Third-Party Auth provider",
		Long:  "Add a new Third-Party Auth provider. Use --oidc-issuer-url for standard OIDC, or --jwks-kid/--jwks-algorithm/--jwks-public-key for custom JWKS.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to add a third-party provider to?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			params := authtp.AddProviderParams{
				OIDCIssuerURL:       authTPAddOIDCIssuerURL,
				CustomJWKSKid:       authTPAddJWKSKid,
				CustomJWKSAlgorithm: authTPAddJWKSAlgorithm,
				CustomJWKSPublicKey: authTPAddJWKSPublicKey,
			}
			return authtp.RunVolcengineAdd(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authTPAddBranch, params)
		},
	}

	authTPRemoveBranch string

	authTPRemoveCmd = &cobra.Command{
		Use:   "remove <provider-id>",
		Short: "Remove a Third-Party Auth provider",
		Long:  "Remove a Third-Party Auth provider by ID. Requests using this provider's JWT will immediately fail.",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to remove the third-party provider from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authtp.RunVolcengineRemove(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authTPRemoveBranch, args[0])
		},
	}

	authTPSyncBranch string

	authTPSyncCmd = &cobra.Command{
		Use:   "sync",
		Short: "Sync Third-Party Auth provider JWKS keys",
		Long:  "Trigger an immediate JWKS public key sync for all Third-Party Auth providers. Use after JWKS key rotation.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRunVolcengineAuth(cmd, "Which project do you want to sync third-party providers for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := volcengine.LoadConfigFromEnv()
			if err != nil {
				return err
			}
			return authtp.RunVolcengineSync(cmd.Context(), volcengine.NewClient(cfg), flags.ProjectRef, authTPSyncBranch)
		},
	}
)

func preRunVolcengineAuth(cmd *cobra.Command, prompt string) error {
	linked := cmd.Flags().Changed("linked")
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
	if linked && workspace {
		return errors.New("--linked cannot be combined with --workspace-id or --project-ref")
	}
	return loadOrPromptVolcengineProjectRef(cmd.Context(), afero.NewOsFs(), prompt)
}

func init() {
	authCmd.PersistentFlags().StringVar(&flags.ProjectRef, "project-ref", "", "Project ref of the Supabase project. Alias of --workspace-id. Defaults to linked project if omitted.")

	// auth config get
	authConfigGetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authConfigGetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authConfigGetCmd.Flags().StringVar(&authConfigGetBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	authConfigGetCmd.Flags().StringSliceVar(&authConfigGetKeys, "key", nil, "Only output specified config key(s). Supports substring match.")

	// auth config set
	authConfigSetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authConfigSetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authConfigSetCmd.Flags().StringVar(&authConfigSetBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	authConfigSetCmd.Flags().StringVar(&authConfigSetJSON, "from-json", "", "Read config patch from a JSON file.")

	// auth hooks get
	authHooksGetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authHooksGetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authHooksGetCmd.Flags().StringVar(&authHooksGetBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	authHooksGetCmd.Flags().StringSliceVar(&authHooksGetKeys, "key", nil, "Only output specified hooks config key(s). Supports substring match.")

	// auth hooks set
	authHooksSetCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authHooksSetCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authHooksSetCmd.Flags().StringVar(&authHooksSetBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	authHooksSetCmd.Flags().StringVar(&authHooksSetJSON, "from-json", "", "Read hooks config patch from a JSON file.")

	// auth third-party list
	authTPListCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authTPListCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authTPListCmd.Flags().StringVar(&authTPListBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")

	// auth third-party add
	authTPAddCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authTPAddCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authTPAddCmd.Flags().StringVar(&authTPAddBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")
	authTPAddCmd.Flags().StringVar(&authTPAddOIDCIssuerURL, "oidc-issuer-url", "", "OIDC Issuer URL of the third-party provider.")
	authTPAddCmd.Flags().StringVar(&authTPAddJWKSKid, "jwks-kid", "", "Custom JWKS Key ID.")
	authTPAddCmd.Flags().StringVar(&authTPAddJWKSAlgorithm, "jwks-algorithm", "", "Custom JWKS algorithm (e.g. RS256, ES256).")
	authTPAddCmd.Flags().StringVar(&authTPAddJWKSPublicKey, "jwks-public-key", "", "Custom JWKS public key (PEM format).")
	authTPAddCmd.MarkFlagsMutuallyExclusive("oidc-issuer-url", "jwks-kid")

	// auth third-party remove
	authTPRemoveCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authTPRemoveCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authTPRemoveCmd.Flags().StringVar(&authTPRemoveBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")

	// auth third-party sync
	authTPSyncCmd.Flags().Bool("linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	authTPSyncCmd.Flags().StringVar(&flags.ProjectRef, "workspace-id", "", "Workspace ID of the Volcengine Supabase project. Defaults to linked project if omitted.")
	authTPSyncCmd.Flags().StringVar(&authTPSyncBranch, "branch-id", "", "Branch ID of the Volcengine Supabase project. Defaults to the workspace default branch.")

	authThirdPartyCmd.AddCommand(authTPListCmd)
	authThirdPartyCmd.AddCommand(authTPAddCmd)
	authThirdPartyCmd.AddCommand(authTPRemoveCmd)
	authThirdPartyCmd.AddCommand(authTPSyncCmd)

	authConfigCmd.AddCommand(authConfigGetCmd)
	authConfigCmd.AddCommand(authConfigSetCmd)
	authHooksCmd.AddCommand(authHooksGetCmd)
	authHooksCmd.AddCommand(authHooksSetCmd)
	authCmd.AddCommand(authConfigCmd)
	authCmd.AddCommand(authHooksCmd)
	authCmd.AddCommand(authThirdPartyCmd)
	rootCmd.AddCommand(authCmd)
}
