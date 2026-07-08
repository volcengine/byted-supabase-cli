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
	"os"

	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

var (
	loginProfile         string
	loginRegion          string
	loginRemote          bool
	loginEndpointURL     string
	loginSkipRegion      bool
	loginCredentialFile  string
	loginIsAgentPlan     bool
	loginAgentPlanSeatID string

	loginCmd = &cobra.Command{
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
				loginIsAgentPlan,
				cmd.Flags().Changed("agent-plan-seat-id"),
				loginAgentPlanSeatID,
			)
			if err != nil {
				return err
			}
			var agentPlanProfilePtr *volcengine.SupabaseProfileConfig
			if changed {
				agentPlanProfilePtr = &agentPlanProfile
			}
			return volcengine.RunConsoleLogin(cmd.Context(), volcengine.ConsoleLoginParams{
				Profile:         loginProfile,
				Region:          loginRegion,
				Remote:          loginRemote,
				EndpointURL:     loginEndpointURL,
				SkipRegion:      loginSkipRegion,
				CredentialFile:  loginCredentialFile,
				AgentPlanConfig: agentPlanProfilePtr,
			}, os.Stdin, os.Stdout)
		},
	}
)

func init() {
	loginFlags := loginCmd.Flags()
	loginFlags.StringVarP(&loginProfile, "profile", "p", "default", "Volcengine profile name.")
	loginFlags.StringVarP(&loginRegion, "region", "r", "", "Volcengine region. Prompts when omitted; empty input defaults to cn-beijing.")
	loginFlags.BoolVar(&loginRemote, "remote", false, "Enable cross-device remote login mode.")
	loginFlags.StringVar(&loginEndpointURL, "endpoint-url", volcengine.DefaultConsoleEndpoint, "Override signin service endpoint URL.")
	loginFlags.BoolVar(&loginSkipRegion, "skip-region", false, "Authenticate without prompting for or saving a default Volcengine region.")
	loginFlags.StringVar(&loginCredentialFile, "credential-file", "", "Import a Volcengine Console Login cache file into this profile.")
	loginFlags.BoolVar(&loginIsAgentPlan, "is-agent-plan", false, "Use Agent Plan by default when creating workspaces with this profile.")
	loginFlags.StringVar(&loginAgentPlanSeatID, "agent-plan-seat-id", "", "Default Agent Plan seat ID for enterprise edition.")
	rootCmd.AddCommand(loginCmd)
}
