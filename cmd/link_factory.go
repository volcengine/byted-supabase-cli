// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/link"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type linkOptions struct {
	WorkspaceID    string
	ProjectName    string
	SupabaseURL    string
	AnonKey        string
	ServiceRoleKey string
	Yes            bool
}

type linkIntent string

const (
	linkIntentManagement      linkIntent = "management"
	linkIntentDirectDataPlane linkIntent = "direct-data-plane"
	linkIntentStatus          linkIntent = "status"
)

func linkIntentFromArgs() linkIntent {
	args := os.Args[1:]
	hasManagement := false
	hasDirect := false
	for _, arg := range args {
		switch arg {
		case "--workspace-id", "--project-ref":
			hasManagement = true
		case "--supabase-url", "--anon-key", "--service-role-key":
			hasDirect = true
		}
	}
	if hasDirect && !hasManagement {
		return linkIntentDirectDataPlane
	}
	return linkIntentManagement
}

func newManagementLinkCommand(ctx RuntimeContext) *cobra.Command {
	var options linkOptions
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "link",
		Short:   "Link this project to a Volcengine Supabase branch",
		Long: `Link this local project to a Volcengine Supabase branch.

There are two ways to link your local project, depending on the credentials and
information available to you.

Option 1: Link using a Volcengine account
  If you have access to a Volcengine account through SSO or an Access Key/Secret
  Key (AK/SK), sign in first and then link the project using a workspace ID. The
  CLI automatically resolves the branch URL and API keys.

  When you link the project this way, management commands - such as those for
  branches, compute resources, authentication, and storage - are also available.

    # Sign in with SSO
    byted-supabase-cli login

    # Alternatively, configure AK/SK credentials
    byted-supabase-cli configure set \
      --access-key <ak> \
      --secret-key <sk> \
      --region <region>

    # Link the project
    byted-supabase-cli link --workspace-id <workspace-id>

  A workspace ID is required. Use --workspace-id or --project-ref.

Option 2: Link using a branch URL and API keys
  Use this option if someone else created the Supabase branch and provided you
  with its connection details.

  The CLI connects directly to the branch, so no Volcengine account is
  required. Management commands are unavailable when you link this way, but
  local-development and branch-level commands - such as db, functions, storage,
  and gen - remain available.

    byted-supabase-cli link \
      --supabase-url <url> \
      --service-role-key <service-role-key>

  The service role key is required for direct linking and serves admin operations.
  The anon key is optional and serves end-user requests when provided.

The flags you provide determine how the project is linked. The options shown
below apply to linking by workspace ID. To view the options for direct branch
linking, run:

  byted-supabase-cli link --supabase-url ...

To check the current link status, run:

  byted-supabase-cli link status

To remove the link, run:

  byted-supabase-cli unlink`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureVolcengineRegion(cmd.Context(), ctx.FS, false); err != nil {
				return err
			}
			client, err := volcengineClientFromAuth(cmd)
			if err != nil {
				return err
			}
			if err := parseVolcengineWorkspaceIDFactory(cmd.Context(), client, &options); err != nil {
				return err
			}
			if err := flags.LoadConfig(ctx.FS); err != nil {
				return err
			}
			return link.RunVolcengine(cmd.Context(), options.WorkspaceID, ctx.FS)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			printLinkSuccess(ctx.Output, options)
		},
	}
	cmd.AddCommand(newLinkStatusCommand(ctx))
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Alias of --workspace-id for compatibility with Supabase CLI project ref.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("project-ref"))
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID of the Volcengine Supabase project.")
	markFlagTelemetrySafe(cmd.Flags().Lookup("workspace-id"))
	cmd.MarkFlagsOneRequired("workspace-id", "project-ref")
	cmd.MarkFlagsMutuallyExclusive("workspace-id", "project-ref")
	return cmd
}

func newDirectDataPlaneLinkCommand(ctx RuntimeContext) *cobra.Command {
	var options linkOptions
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "link",
		Short:   "Link this project directly to a Supabase branch",
		Long: `Link this local project directly to a Supabase branch using its URL and
API keys. No Volcengine account is required.

Use this option if someone else created the Supabase branch and provided you
with its connection details. Management commands are unavailable when you link
this way, but local-development and branch-level commands - such as db,
functions, storage, and gen - remain available.

  byted-supabase-cli link \
    --supabase-url <url> \
    --service-role-key <service-role-key>

The service role key is required for direct linking and serves admin operations.
The anon key is optional and serves end-user requests when provided.

To check the current link status, run:

  byted-supabase-cli link status

To remove the link, run:

  byted-supabase-cli unlink`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(options.SupabaseURL) == "" {
				return errors.New("linking with a branch URL and API keys requires --supabase-url")
			}
			return runSoloLinkFactory(options, ctx.FS)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			printLinkSuccess(ctx.Output, options)
		},
	}
	cmd.AddCommand(newLinkStatusCommand(ctx))
	SetAuthPolicy(cmd, AuthPolicyNone)
	cmd.Flags().StringVar(&options.SupabaseURL, "supabase-url", "", "URL of the Supabase instance.")
	cmd.Flags().StringVar(&options.AnonKey, "anon-key", "", "Anon key of the Supabase instance.")
	cmd.Flags().StringVar(&options.ServiceRoleKey, "service-role-key", "", "Service role key of the Supabase instance.")
	cmd.Flags().BoolVar(&options.Yes, "yes", false, "replace the current link without prompting")
	_ = cmd.MarkFlagRequired("supabase-url")
	_ = cmd.MarkFlagRequired("service-role-key")
	return cmd
}

func newLinkCommand(ctx RuntimeContext) *cobra.Command {
	return newManagementLinkCommand(ctx)
}

func newSoloLinkCommand(ctx RuntimeContext) *cobra.Command {
	return newDirectDataPlaneLinkCommand(ctx)
}

func newLinkStatusCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current project link",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinkStatus(ctx)
		},
	}
	SetAuthPolicy(cmd, AuthPolicyNone)
	return cmd
}

type linkStatus struct {
	Mode                 string `json:"mode"`
	Linked               bool   `json:"linked"`
	Type                 string `json:"type,omitempty"`
	SupabaseURL          string `json:"supabase_url,omitempty"`
	WorkspaceID          string `json:"workspace_id,omitempty"`
	BranchID             string `json:"branch_id,omitempty"`
	AnonKeyConfigured    bool   `json:"anon_key_configured,omitempty"`
	ServiceKeyConfigured bool   `json:"service_role_key_configured,omitempty"`
	DatabaseConfigured   bool   `json:"database_url_configured,omitempty"`
	SoloConfigValid      bool   `json:"solo_config_valid,omitempty"`
	SoloConfigError      string `json:"solo_config_error,omitempty"`
	ConfigSource         string `json:"config_source,omitempty"`
	Legacy               bool   `json:"legacy,omitempty"`
}

func runLinkStatus(ctx RuntimeContext) error {
	projectDir := runtimeWorkdirFromArgs()
	if resolution, resolutionErr := soloconfig.NewResolver(projectDir).Resolve(); resolutionErr == nil {
		result := linkStatus{
			Mode: "solo", Linked: true, Type: string(link.LinkTypeSolo),
			SupabaseURL:          resolution.Profile.SupabaseURL,
			ConfigSource:         string(resolution.Source),
			SoloConfigValid:      true,
			AnonKeyConfigured:    resolution.Profile.AnonKey != "",
			ServiceKeyConfigured: resolution.Profile.ServiceRoleKey != "",
			DatabaseConfigured:   false,
		}
		if resolution.Source == soloconfig.ConfigSourceProfile {
			result.WorkspaceID = resolution.State.WorkspaceID
			result.BranchID = resolution.State.BranchID
		}
		return renderLinkStatus(ctx, result)
	}
	if _, configured, envErr := soloconfig.LoadEnvironmentProfile(); configured {
		result := linkStatus{
			Mode: "solo", Linked: false, Type: string(link.LinkTypeSolo),
			ConfigSource:    string(soloconfig.ConfigSourceEnvironment),
			SoloConfigError: envErr.Error(),
		}
		if state, stateErr := link.LoadStateAtDir(projectDir); stateErr == nil && state.Type == link.LinkTypeSolo {
			result.ConfigSource = string(soloconfig.ConfigSourceProfile)
		}
		return renderLinkStatus(ctx, result)
	}
	state, err := link.LoadState(ctx.FS)
	if err == nil {
		result := linkStatus{
			Mode:        string(state.Type),
			Linked:      true,
			Type:        string(state.Type),
			SupabaseURL: state.SupabaseURL,
			WorkspaceID: state.WorkspaceID,
			BranchID:    state.BranchID,
		}
		if state.Type == link.LinkTypeSolo {
			if profileErr := soloconfig.NewResolver(projectDir).Validate(); profileErr != nil {
				result.SoloConfigError = profileErr.Error()
			} else {
				result.SoloConfigValid = true
			}
			result.ConfigSource = string(soloconfig.ConfigSourceProfile)
			if profile, profileErr := soloconfig.LoadCurrentProfile(); profileErr == nil {
				result.AnonKeyConfigured = strings.TrimSpace(profile.AnonKey) != ""
				result.ServiceKeyConfigured = strings.TrimSpace(profile.ServiceRoleKey) != ""
				result.DatabaseConfigured = false
			}
		}
		return renderLinkStatus(ctx, result)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return renderLinkStatus(ctx, linkStatus{
		Mode: "management", Linked: false,
	})
}

func renderLinkStatus(ctx RuntimeContext, result linkStatus) error {
	if utils.OutputFormat.Value != utils.OutputPretty {
		return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
	}
	if result.Type == string(link.LinkTypeSolo) && !result.SoloConfigValid && result.SoloConfigError != "" {
		_, err := fmt.Fprintf(ctx.Output, "The branch link configuration is invalid.\nReason: %s\nPlease set the required environment variables or run `byted-supabase-cli link` again.\n", result.SoloConfigError)
		return err
	}
	if !result.Linked {
		if result.Legacy {
			_, err := fmt.Fprintln(ctx.Output, "Linked via: branch URL and keys (legacy state; no project link is present).")
			return err
		}
		_, err := fmt.Fprintln(ctx.Output, "Link status: not linked.")
		return err
	}
	if result.Type == string(link.LinkTypeSolo) {
		_, err := fmt.Fprintf(ctx.Output,
			"Linked via: branch URL and keys\nSupabase URL: %s\nAnon Key: %s\nService Role Key: %s\nDatabase URL: %s\nConfig Source: %s\nLink config: %s\n",
			result.SupabaseURL, configuredLabel(result.AnonKeyConfigured),
			configuredLabel(result.ServiceKeyConfigured), configuredLabel(result.DatabaseConfigured),
			firstNonEmpty(result.ConfigSource, "profile"), soloConfigLabel(result))
		return err
	}
	_, err := fmt.Fprintf(ctx.Output,
		"Linked via: Volcengine account\nWorkspace ID: %s\nBranch ID:    %s\n",
		result.WorkspaceID, result.BranchID)
	return err
}

func soloConfigLabel(result linkStatus) string {
	if result.SoloConfigValid {
		return "valid"
	}
	if result.SoloConfigError != "" {
		return "invalid: " + result.SoloConfigError
	}
	return "not checked"
}

func configuredLabel(configured bool) string {
	if configured {
		return "configured"
	}
	return "not configured"
}

func printLinkSuccess(output io.Writer, options linkOptions) {
	if strings.TrimSpace(options.SupabaseURL) != "" {
		fmt.Fprintf(output, "Supabase branch linked successfully:\n  %s\n\n", strings.TrimRight(strings.TrimSpace(options.SupabaseURL), "/"))
		fmt.Fprintln(output, "Command list has changed...")
		fmt.Fprintln(output, "\nView status:")
		fmt.Fprintln(output, "  byted-supabase-cli link status")
		fmt.Fprintln(output, "\nTo go back to managing cloud resources with your Volcengine account:")
		fmt.Fprintln(output, "  byted-supabase-cli unlink")
		return
	}
	if strings.TrimSpace(options.WorkspaceID) != "" {
		fmt.Fprintf(output, "Linked workspace %q with your Volcengine account.\n", strings.TrimSpace(options.WorkspaceID))
		fmt.Fprintln(output, "Management commands will be available on the next invocation.")
		fmt.Fprintln(output, "To link a branch directly with its URL and keys, run `byted-supabase-cli link --supabase-url <url> ...`.")
		return
	}
	fmt.Fprintln(output, "Link completed.")
}

func runSoloLinkFactory(options linkOptions, fsys afero.Fs) error {
	if strings.TrimSpace(options.ServiceRoleKey) == "" {
		return errors.New("missing --service-role-key")
	}
	if anonKey := strings.TrimSpace(options.AnonKey); anonKey != "" {
		if err := probeSoloKey(options.SupabaseURL, "anon", anonKey); err != nil {
			return err
		}
	}
	if serviceRoleKey := strings.TrimSpace(options.ServiceRoleKey); serviceRoleKey != "" {
		if err := probeSoloKey(options.SupabaseURL, "service_role", serviceRoleKey); err != nil {
			return err
		}
	}
	if state, err := link.LoadState(fsys); err == nil {
		if !options.Yes {
			return errors.Errorf("a %s link is already active; rerun with --yes to replace it", state.Type)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	credentialRef := soloCredentialRef(options.SupabaseURL)
	if err := soloconfig.SaveProfile(soloconfig.Profile{
		Name:           credentialRef,
		SupabaseURL:    options.SupabaseURL,
		AnonKey:        options.AnonKey,
		ServiceRoleKey: options.ServiceRoleKey,
	}); err != nil {
		return err
	}
	if err := link.SaveSoloState(fsys, options.SupabaseURL, credentialRef); err != nil {
		return err
	}
	return nil
}

func probeSoloKey(supabaseURL, keyName, key string) error {
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(supabaseURL), "/"))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return errors.Errorf("invalid Supabase URL %q", supabaseURL)
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/rest/v1/"

	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
	if err != nil {
		return errors.Errorf("failed to validate %s key: %w", keyName, err)
	}
	request.Header.Set("apikey", key)
	request.Header.Set("Authorization", "Bearer "+key)
	response, err := client.Do(request)
	if err != nil {
		return errors.Errorf("failed to validate %s key against Supabase URL: %w", keyName, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.Errorf("failed to validate %s key against Supabase URL: REST API returned HTTP %d, expected HTTP 200", keyName, response.StatusCode)
	}
	return nil
}

func soloCredentialRef(supabaseURL string) string {
	supabaseURL = strings.TrimRight(strings.TrimSpace(supabaseURL), "/")
	sum := sha256.Sum256([]byte(supabaseURL))
	return "link-" + fmt.Sprintf("%x", sum[:8])
}

func parseVolcengineWorkspaceIDFactory(ctx context.Context, client *volcengine.Client, options *linkOptions) error {
	if options.WorkspaceID != "" {
		return nil
	}
	return errors.New("missing workspace id. Supply --workspace-id or --project-ref.")
}
