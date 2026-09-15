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

type authWorkspaceOptions struct {
	WorkspaceID string
	BranchID    string
	Linked      bool
}

func newAuthCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{GroupID: groupManagementAPI, Use: "auth", Short: "Manage Supabase Auth Configuration"}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newAuthConfigCommand(ctx),
		newAuthHooksCommand(ctx),
		newAuthThirdPartyCommand(ctx),
	)
	return cmd
}

func addAuthWorkspaceFlags(cmd *cobra.Command, options *authWorkspaceOptions) {
	cmd.Flags().BoolVar(&options.Linked, "linked", true, "Use the linked workspace when --workspace-id/--project-ref is omitted.")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Project ref alias of --workspace-id.")
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID of the Volcengine Supabase project.")
}

func prepareAuthWorkspace(cmd *cobra.Command, fsys afero.Fs, options *authWorkspaceOptions, prompt string) error {
	workspace := cmd.Flags().Changed("workspace-id") || cmd.Flags().Changed("project-ref")
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

func authClient(cmd *cobra.Command) (*volcengine.Client, error) {
	return volcengineClientFromAuth(cmd)
}

func newAuthConfigCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage Auth general configuration"}
	cmd.AddCommand(
		newAuthConfigGetCommand(ctx),
		newAuthConfigSetCommand(ctx),
	)
	return cmd
}

func newAuthConfigGetCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	var keys []string
	cmd := &cobra.Command{Use: "get", Short: "Get Auth configuration", Long: "Get Auth configuration for the linked project. Returns all config keys by default, or filter with --key.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to get auth config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authconfig.RunVolcengineGet(cmd.Context(), client, options.WorkspaceID, options.BranchID, keys)
		}}
	addAuthWorkspaceFlags(cmd, &options)
	cmd.Flags().StringSliceVar(&keys, "key", nil, "Only output specified config key(s). Supports substring match.")
	return cmd
}

func newAuthConfigSetCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	var fromJSON string
	cmd := &cobra.Command{Use: "set [KEY=VALUE ...]", Short: "Set Auth configuration", Long: "Set Auth configuration for the linked project. Provide KEY=VALUE pairs or --from-json <file>.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to set auth config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authconfig.RunVolcengineSet(cmd.Context(), client, options.WorkspaceID, options.BranchID, args, fromJSON, ctx.FS)
		}}
	addAuthWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&fromJSON, "from-json", "", "Read config patch from a JSON file.")
	return cmd
}

func newAuthHooksCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "hooks", Short: "Manage Auth Hooks configuration"}
	cmd.AddCommand(newAuthHooksGetCommand(ctx), newAuthHooksSetCommand(ctx))
	return cmd
}

func newAuthHooksGetCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	var keys []string
	cmd := &cobra.Command{Use: "get", Short: "Get Auth Hooks configuration", Long: "Get Auth Hooks configuration for the linked project. Returns all hooks config keys by default, or filter with --key.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to get hooks config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authhooks.RunVolcengineGet(cmd.Context(), client, options.WorkspaceID, options.BranchID, keys)
		}}
	addAuthWorkspaceFlags(cmd, &options)
	cmd.Flags().StringSliceVar(&keys, "key", nil, "Only output specified hooks config key(s). Supports substring match.")
	return cmd
}

func newAuthHooksSetCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	var fromJSON string
	cmd := &cobra.Command{Use: "set [KEY=VALUE ...]", Short: "Set Auth Hooks configuration", Long: "Set Auth Hooks configuration for the linked project. Provide KEY=VALUE pairs or --from-json <file>.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to set hooks config for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authhooks.RunVolcengineSet(cmd.Context(), client, options.WorkspaceID, options.BranchID, args, fromJSON, ctx.FS)
		}}
	addAuthWorkspaceFlags(cmd, &options)
	cmd.Flags().StringVar(&fromJSON, "from-json", "", "Read hooks config patch from a JSON file.")
	return cmd
}

func newAuthThirdPartyCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "third-party", Aliases: []string{"tp"}, Short: "Manage Third-Party Auth providers"}
	cmd.AddCommand(
		newAuthTPListCommand(ctx),
		newAuthTPAddCommand(ctx),
		newAuthTPRemoveCommand(ctx),
		newAuthTPSyncCommand(ctx),
	)
	return cmd
}

func newAuthTPListCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	cmd := &cobra.Command{Use: "list", Short: "List Third-Party Auth providers", Long: "List all configured Third-Party Auth providers for the linked project.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to list third-party providers for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authtp.RunVolcengineList(cmd.Context(), client, options.WorkspaceID, options.BranchID)
		}}
	addAuthWorkspaceFlags(cmd, &options)
	return cmd
}

type authTPAddOptions struct {
	authWorkspaceOptions
	OIDCIssuerURL string
	JWKSKid       string
	JWKSAlgorithm string
	JWKSPublicKey string
}

func newAuthTPAddCommand(ctx RuntimeContext) *cobra.Command {
	var options authTPAddOptions
	cmd := &cobra.Command{Use: "add", Short: "Add a Third-Party Auth provider", Long: "Add a new Third-Party Auth provider. Use --oidc-issuer-url for standard OIDC, or --jwks-kid/--jwks-algorithm/--jwks-public-key for custom JWKS.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options.authWorkspaceOptions, "Which project do you want to add a third-party provider to?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authtp.RunVolcengineAdd(cmd.Context(), client, options.WorkspaceID, options.BranchID, authtp.AddProviderParams{
				OIDCIssuerURL: options.OIDCIssuerURL, CustomJWKSKid: options.JWKSKid,
				CustomJWKSAlgorithm: options.JWKSAlgorithm, CustomJWKSPublicKey: options.JWKSPublicKey,
			})
		}}
	addAuthWorkspaceFlags(cmd, &options.authWorkspaceOptions)
	cmd.Flags().StringVar(&options.OIDCIssuerURL, "oidc-issuer-url", "", "OIDC Issuer URL of the third-party provider.")
	cmd.Flags().StringVar(&options.JWKSKid, "jwks-kid", "", "Custom JWKS Key ID.")
	cmd.Flags().StringVar(&options.JWKSAlgorithm, "jwks-algorithm", "", "Custom JWKS algorithm (e.g. RS256, ES256).")
	cmd.Flags().StringVar(&options.JWKSPublicKey, "jwks-public-key", "", "Custom JWKS public key (PEM format).")
	cmd.MarkFlagsMutuallyExclusive("oidc-issuer-url", "jwks-kid")
	return cmd
}

func newAuthTPRemoveCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	cmd := &cobra.Command{Use: "remove <provider-id>", Short: "Remove a Third-Party Auth provider", Long: "Remove a Third-Party Auth provider by ID. Requests using this provider's JWT will immediately fail.", Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to remove the third-party provider from?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authtp.RunVolcengineRemove(cmd.Context(), client, options.WorkspaceID, options.BranchID, args[0])
		}}
	addAuthWorkspaceFlags(cmd, &options)
	return cmd
}

func newAuthTPSyncCommand(ctx RuntimeContext) *cobra.Command {
	var options authWorkspaceOptions
	cmd := &cobra.Command{Use: "sync", Short: "Sync Third-Party Auth provider JWKS keys", Long: "Trigger an immediate JWKS public key sync for all Third-Party Auth providers. Use after JWKS key rotation.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return prepareAuthWorkspace(cmd, ctx.FS, &options, "Which project do you want to sync third-party providers for?")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authClient(cmd)
			if err != nil {
				return err
			}
			return authtp.RunVolcengineSync(cmd.Context(), client, options.WorkspaceID, options.BranchID)
		}}
	addAuthWorkspaceFlags(cmd, &options)
	return cmd
}
