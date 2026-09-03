// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type loginOptions struct {
	Profile         string
	Region          string
	Remote          bool
	EndpointURL     string
	SkipRegion      bool
	CredentialFile  string
	IsAgentPlan     bool
	AgentPlanSeatID string
}

func newLoginCommand(ctx RuntimeContext) *cobra.Command {
	var options loginOptions
	cmd := &cobra.Command{
		GroupID: groupLocalDev,
		Use:     "login",
		Short:   "Authenticate with Volcengine",
		Long: `Authenticate with Volcengine Console using OAuth 2.0 + PKCE.
Opens a browser for authentication and caches temporary STS credentials locally.

Supports three modes:
  - Local (default): Opens browser on the same device
  - Remote (--remote): For headless environments, displays URL and accepts code input
  - Credential file (--credential-file): Imports an existing Console Login cache

Region is only used as the default region for subsequent Volcengine API calls.
Use --skip-region to authenticate without saving a profile region.`,
		Example: `byted-supabase-cli login
byted-supabase-cli login --profile dev --region cn-beijing
byted-supabase-cli login --profile dev --region cn-beijing --remote
byted-supabase-cli login --credential-file /path/to/cache.json --profile dev
byted-supabase-cli login --skip-region`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			agentPlanProfile, changed, err := resolveAgentPlanProfileUpdate(
				cmd.Flags().Changed("is-agent-plan"),
				options.IsAgentPlan,
				cmd.Flags().Changed("agent-plan-seat-id"),
				options.AgentPlanSeatID,
			)
			if err != nil {
				return err
			}
			var agentPlanProfilePtr *volcengine.SupabaseProfileConfig
			if changed {
				agentPlanProfilePtr = &agentPlanProfile
			}
			input := ctx.Input
			if input == nil {
				input = os.Stdin
			}
			return volcengine.RunConsoleLogin(cmd.Context(), volcengine.ConsoleLoginParams{
				Profile:         options.Profile,
				Region:          options.Region,
				Remote:          options.Remote,
				EndpointURL:     options.EndpointURL,
				SkipRegion:      options.SkipRegion,
				CredentialFile:  options.CredentialFile,
				AgentPlanConfig: agentPlanProfilePtr,
			}, input, ctx.Output)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&options.Profile, "profile", "p", "default", "Volcengine profile name.")
	flags.StringVarP(&options.Region, "region", "r", "", "Volcengine region. Prompts when omitted; empty input defaults to cn-beijing.")
	flags.BoolVar(&options.Remote, "remote", false, "Enable cross-device remote login mode.")
	flags.StringVar(&options.EndpointURL, "endpoint-url", volcengine.DefaultConsoleEndpoint, "Override signin service endpoint URL.")
	flags.BoolVar(&options.SkipRegion, "skip-region", false, "Authenticate without prompting for or saving a default Volcengine region.")
	flags.StringVar(&options.CredentialFile, "credential-file", "", "Import a Volcengine Console Login cache file into this profile.")
	flags.BoolVar(&options.IsAgentPlan, "is-agent-plan", false, "Use Agent Plan by default when creating workspaces with this profile.")
	flags.StringVar(&options.AgentPlanSeatID, "agent-plan-seat-id", "", "Default Agent Plan seat ID for enterprise edition.")
	return cmd
}
