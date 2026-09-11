package solo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func newAuthCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	auth := &cobra.Command{
		GroupID: "data-plane",
		Use:     "auth",
		Short:   "Manage Supabase Auth",
	}
	auth.AddCommand(newAuthConfigCommand(ctx), newAuthHooksCommand(ctx), newAuthThirdPartyCommand(ctx))
	return auth
}

func newAuthHooksCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{Use: "hooks", Short: "Manage Auth Hooks configuration"}
	command.AddCommand(newAuthHooksGetCommand(ctx), newAuthHooksSetCommand(ctx))
	return command
}

func newAuthHooksGetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var keys []string
	command := &cobra.Command{
		Use:   "get",
		Short: "Get Auth Hooks configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			config, err := client.GetHooks(cmd.Context())
			if err != nil {
				return err
			}
			config, err = filterConfig(config, keys)
			if err != nil {
				return err
			}
			return writeJSON(ctx, config)
		},
	}
	command.Flags().StringSliceVar(&keys, "key", nil, "Only output specified hooks config key(s). Supports substring match.")
	return command
}

func newAuthHooksSetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var fromJSON string
	command := &cobra.Command{
		Use:   "set [KEY=VALUE ...]",
		Short: "Set Auth Hooks configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := buildConfigPatch(args, fromJSON, ctx.FS)
			if err != nil {
				return err
			}
			if len(patch) == 0 {
				return fmt.Errorf("no hooks config values provided. Use KEY=VALUE arguments or --from-json <file>.")
			}
			if err := confirmSoloWrite(cmd, "set Auth Hooks configuration"); err != nil {
				return err
			}
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			result, err := client.SetHooks(cmd.Context(), patch)
			if err != nil {
				return err
			}
			return writeJSON(ctx, result)
		},
	}
	command.Flags().StringVar(&fromJSON, "from-json", "", "Read hooks config patch from a JSON file.")
	return command
}

func newAuthThirdPartyCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{
		Use:     "third-party",
		Aliases: []string{"tp"},
		Short:   "Manage Third-Party Auth providers",
	}
	command.AddCommand(
		newAuthThirdPartyListCommand(ctx),
		newAuthThirdPartyAddCommand(ctx),
		newAuthThirdPartyRemoveCommand(ctx),
		newAuthThirdPartySyncCommand(ctx),
	)
	return command
}

func newAuthThirdPartyListCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Third-Party Auth providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			providers, err := client.ListThirdParty(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(ctx, providers)
		},
	}
}

func newAuthThirdPartyAddCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var oidcIssuerURL, jwksKID, jwksAlgorithm, jwksPublicKey string
	command := &cobra.Command{
		Use:   "add",
		Short: "Add a Third-Party Auth provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := confirmSoloWrite(cmd, "add a Third-Party Auth provider"); err != nil {
				return err
			}
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			result, err := client.AddThirdParty(cmd.Context(), soloconfig.AddThirdPartyParams{
				OIDCIssuerURL: oidcIssuerURL, CustomJWKSKid: jwksKID,
				CustomJWKSAlgorithm: jwksAlgorithm, CustomJWKSPublicKey: jwksPublicKey,
			})
			if err != nil {
				return err
			}
			return writeJSON(ctx, result)
		},
	}
	command.Flags().StringVar(&oidcIssuerURL, "oidc-issuer-url", "", "OIDC Issuer URL of the third-party provider.")
	command.Flags().StringVar(&jwksKID, "jwks-kid", "", "Custom JWKS Key ID.")
	command.Flags().StringVar(&jwksAlgorithm, "jwks-algorithm", "", "Custom JWKS algorithm (e.g. RS256, ES256).")
	command.Flags().StringVar(&jwksPublicKey, "jwks-public-key", "", "Custom JWKS public key (PEM format).")
	command.MarkFlagsMutuallyExclusive("oidc-issuer-url", "jwks-kid")
	return command
}

func newAuthThirdPartyRemoveCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <provider-id>",
		Short: "Remove a Third-Party Auth provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := confirmSoloWrite(cmd, "remove Auth provider "+args[0]); err != nil {
				return err
			}
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			if err := client.RemoveThirdParty(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeJSON(ctx, map[string]string{"id": args[0], "status": "removed"})
		},
	}
}

func newAuthThirdPartySyncCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync Third-Party Auth provider JWKS keys",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := confirmSoloWrite(cmd, "sync Auth provider keys"); err != nil {
				return err
			}
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			if err := client.SyncThirdParty(cmd.Context()); err != nil {
				return err
			}
			return writeJSON(ctx, map[string]string{"status": "synced"})
		},
	}
}

func newAuthConfigCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	config := &cobra.Command{
		Use:   "config",
		Short: "Manage Auth configuration",
	}
	config.AddCommand(newAuthConfigGetCommand(ctx), newAuthConfigSetCommand(ctx))
	return config
}

func newAuthConfigGetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var keys []string
	command := &cobra.Command{
		Use:   "get",
		Short: "Get Auth configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			config, err := client.GetConfig(cmd.Context())
			if err != nil {
				return err
			}
			config, err = filterConfig(config, keys)
			if err != nil {
				return err
			}
			return writeJSON(ctx, config)
		},
	}
	command.Flags().StringSliceVar(&keys, "key", nil, "Only output specified config key(s). Supports substring match.")
	return command
}

func newAuthConfigSetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var fromJSON string
	command := &cobra.Command{
		Use:   "set [KEY=VALUE ...]",
		Short: "Set Auth configuration",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := buildConfigPatch(args, fromJSON, ctx.FS)
			if err != nil {
				return err
			}
			if len(patch) == 0 {
				return fmt.Errorf("no config values provided. Use KEY=VALUE arguments or --from-json <file>.")
			}
			if err := confirmSoloWrite(cmd, "set Auth configuration"); err != nil {
				return err
			}
			client, err := newAuthClient()
			if err != nil {
				return err
			}
			result, err := client.SetConfig(cmd.Context(), patch)
			if err != nil {
				return err
			}
			return writeJSON(ctx, result)
		},
	}
	command.Flags().StringVar(&fromJSON, "from-json", "", "Read config patch from a JSON file.")
	return command
}

func newAuthClient() (*soloconfig.AuthClient, error) {
	profile, err := soloconfig.LoadCurrentProfile()
	if err != nil {
		return nil, err
	}
	return soloconfig.NewAuthClient(profile)
}

func buildConfigPatch(pairs []string, fromJSON string, fsys afero.Fs) (map[string]interface{}, error) {
	patch := make(map[string]interface{})
	if fromJSON != "" {
		data, err := afero.ReadFile(fsys, fromJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read JSON file %q: %w", fromJSON, err)
		}
		if err := json.Unmarshal(data, &patch); err != nil {
			return nil, fmt.Errorf("failed to parse JSON file %q: %w", fromJSON, err)
		}
	}
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		index := strings.IndexByte(pair, '=')
		if index < 1 {
			return nil, fmt.Errorf("invalid config pair %q: expected KEY=VALUE format", pair)
		}
		key, rawValue := pair[:index], pair[index+1:]
		var value interface{}
		if err := json.Unmarshal([]byte(rawValue), &value); err != nil {
			value = rawValue
		}
		patch[key] = value
	}
	return patch, nil
}

func filterConfig(config map[string]interface{}, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return config, nil
	}
	filtered := make(map[string]interface{})
	for key, value := range config {
		for _, filter := range keys {
			if strings.Contains(strings.ToUpper(key), strings.ToUpper(filter)) {
				filtered[key] = value
				break
			}
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no config keys matched filter %v", keys)
	}
	return filtered, nil
}

func writeJSON(ctx commandtree.RuntimeContext, value interface{}) error {
	encoder := json.NewEncoder(ctx.Output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
