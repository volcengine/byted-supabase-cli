// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/aigateway"
	"github.com/volcengine/byted-supabase-cli/internal/projects/apiKeys"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type aiGatewayWorkspaceOptions struct {
	WorkspaceID string
	BranchID    string
}

type aiGatewayTokenOptions struct {
	Token    string
	TokenEnv string
}

type aiGatewayChatOptions struct {
	aiGatewayWorkspaceOptions
	aiGatewayTokenOptions
	Model       string
	Message     string
	System      string
	RequestFile string
	Stream      bool
	Temperature float64
	MaxTokens   int
	TopP        float64
}

type aiGatewayAPIKeysOptions struct {
	aiGatewayWorkspaceOptions
	ProjectName string
	Reveal      bool
}

func aiGatewayCommandGroup() CommandGroup {
	return CommandGroup{
		Factories: []CommandFactory{
			newAIGatewayCommand,
		},
	}
}

func newAIGatewayCommand(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{GroupID: groupManagementAPI, Use: "ai-gateway", Short: "Manage Supabase AI Gateway for a Workspace Branch"}
	SetAuthPolicy(cmd, AuthPolicyVolcengine)
	cmd.AddCommand(
		newAIGatewayStatusCmd(ctx),
		newAIGatewayConfigCmd(ctx),
		newAIGatewayAPIKeysCmd(ctx),
		newAIGatewayModelsCmd(ctx),
		newAIGatewayQuotaCmd(ctx),
		newAIGatewayUsageCmd(ctx),
		newAIGatewayChatCmd(ctx),
		newAIGatewayTestCmd(ctx),
		newAIGatewayInitCmd(ctx),
	)
	return cmd
}

func newAIGatewayQuotaCmd(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "quota", Short: "Manage AI Gateway token quotas"}
	cmd.AddCommand(
		newAIGatewayQuotaGetCmd(ctx),
		newAIGatewayQuotaSetDefaultCmd(ctx),
		newAIGatewayQuotaUsersCmd(ctx),
	)
	return cmd
}

func newAIGatewayQuotaGetCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show the default daily AI Gateway token quota",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, branchID, err := resolveAIGatewayManagementTarget(cmd, &workspace)
			if err != nil {
				return err
			}
			quota, err := client.DescribeAIGlobalQuota(cmd.Context(), workspaceID, branchID)
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]interface{}{
				"workspace_id": workspaceID, "branch_id": branchID,
				"daily_token_limit": quota.DailyTokenLimit,
				"daily_image_limit": quota.DailyImageLimit,
			})
		},
	}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	return cmd
}

func newAIGatewayQuotaSetDefaultCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var tokenLimit, imageLimit int64
	cmd := &cobra.Command{
		Use:   "set-default",
		Short: "Set the default daily AI Gateway token quota",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, branchID, err := resolveAIGatewayManagementTarget(cmd, &workspace)
			if err != nil {
				return err
			}
			var tokenLimitPtr, imageLimitPtr *int64
			if cmd.Flags().Changed("token-limit") {
				tokenLimitPtr = &tokenLimit
			}
			if cmd.Flags().Changed("image-limit") {
				imageLimitPtr = &imageLimit
			}
			if tokenLimitPtr == nil && imageLimitPtr == nil {
				return errors.New("at least one of --token-limit or --image-limit is required")
			}
			if err := client.ModifyAIGlobalQuotaWithLimits(cmd.Context(), workspaceID, branchID, tokenLimitPtr, imageLimitPtr); err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]interface{}{
				"workspace_id": workspaceID, "branch_id": branchID,
				"daily_token_limit": tokenLimitPtr,
				"daily_image_limit": imageLimitPtr,
			})
		},
	}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	cmd.Flags().Int64Var(&tokenLimit, "token-limit", 0, "Daily token limit per user. Use -1 for unlimited.")
	cmd.Flags().Int64Var(&imageLimit, "image-limit", 0, "Daily image generation limit per user. Use -1 for unlimited.")
	return cmd
}

func newAIGatewayQuotaUsersCmd(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "users", Short: "Manage per-user AI Gateway token quotas"}
	cmd.AddCommand(
		newAIGatewayQuotaUsersListCmd(ctx),
		newAIGatewayQuotaUsersGetCmd(ctx),
		newAIGatewayQuotaUsersSetCmd(ctx),
	)
	return cmd
}

func newAIGatewayQuotaUsersListCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List AI Gateway users, quotas, and token usage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateVolcenginePageFlags(limit, offset, false); err != nil {
				return err
			}
			client, workspaceID, branchID, err := resolveAIGatewayManagementTarget(cmd, &workspace)
			if err != nil {
				return err
			}
			result, err := client.DescribeAIUserSummaries(cmd.Context(), volcengine.DescribeAIUserSummariesParams{
				WorkspaceID: workspaceID, BranchID: branchID, Limit: limit, Offset: offset,
			})
			if err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	cmd.Flags().IntVar(&limit, "limit", volcengineDefaultListLimit, "Maximum number of users to return (1-100).")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of users to skip.")
	return cmd
}

func newAIGatewayQuotaUsersGetCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var userID string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show one AI Gateway user's quota and token usage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(userID) == "" {
				return errors.New("missing required flag: --user-id")
			}
			client, workspaceID, branchID, err := resolveAIGatewayManagementTarget(cmd, &workspace)
			if err != nil {
				return err
			}
			for offset := 0; ; offset += 100 {
				result, err := client.DescribeAIUserSummaries(cmd.Context(), volcengine.DescribeAIUserSummariesParams{
					WorkspaceID: workspaceID, BranchID: branchID, Limit: 100, Offset: offset,
				})
				if err != nil {
					return err
				}
				for _, user := range result.Users {
					if user.UserID == userID {
						return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, user)
					}
				}
				if len(result.Users) == 0 || offset+len(result.Users) >= result.Total {
					break
				}
			}
			return fmt.Errorf("AI Gateway user %q not found", userID)
		},
	}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID.")
	return cmd
}

func newAIGatewayQuotaUsersSetCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var userID string
	var tokenLimit, imageLimit int64
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set one AI Gateway user's daily token quota",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(userID) == "" {
				return errors.New("missing required flag: --user-id")
			}
			client, workspaceID, branchID, err := resolveAIGatewayManagementTarget(cmd, &workspace)
			if err != nil {
				return err
			}
			var tokenLimitPtr, imageLimitPtr *int64
			if cmd.Flags().Changed("token-limit") {
				tokenLimitPtr = &tokenLimit
			}
			if cmd.Flags().Changed("image-limit") {
				imageLimitPtr = &imageLimit
			}
			if tokenLimitPtr == nil && imageLimitPtr == nil {
				return errors.New("at least one of --token-limit or --image-limit is required")
			}
			if err := client.ModifyAIUserQuotaWithLimits(cmd.Context(), volcengine.ModifyAIUserQuotaParams{
				WorkspaceID: workspaceID, BranchID: branchID, UserID: userID,
				QuotaCustomized: true, DailyTokenLimit: tokenLimitPtr, DailyImageLimit: imageLimitPtr,
			}); err != nil {
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]interface{}{
				"workspace_id": workspaceID, "branch_id": branchID, "user_id": userID,
				"daily_token_limit": tokenLimitPtr,
				"daily_image_limit": imageLimitPtr,
			})
		},
	}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID.")
	cmd.Flags().Int64Var(&tokenLimit, "token-limit", 0, "Daily token limit. Use -1 for unlimited.")
	cmd.Flags().Int64Var(&imageLimit, "image-limit", 0, "Daily image generation limit. Use -1 for unlimited.")
	return cmd
}

func newAIGatewayUsageCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Show aggregated AI Gateway token usage for a branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, branchID, err := resolveAIGatewayManagementTarget(cmd, &workspace)
			if err != nil {
				return err
			}
			usage := volcengine.AIUsage{WorkspaceID: workspaceID, BranchID: branchID}
			for offset := 0; ; offset += 100 {
				result, err := client.DescribeAIUserSummaries(cmd.Context(), volcengine.DescribeAIUserSummariesParams{
					WorkspaceID: workspaceID, BranchID: branchID, Limit: 100, Offset: offset,
				})
				if err != nil {
					return err
				}
				usage.UserCount = result.Total
				for _, user := range result.Users {
					usage.TodayTokens += user.LastDailyTokens
					usage.TotalTokens += user.TotalTokens
					usage.TodayImages += user.LastDailyImages
					usage.TotalImages += user.TotalImages
				}
				if len(result.Users) == 0 || offset+len(result.Users) >= result.Total {
					break
				}
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, usage)
		},
	}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	return cmd
}

func resolveAIGatewayManagementTarget(cmd *cobra.Command, workspace *aiGatewayWorkspaceOptions) (*volcengine.Client, string, string, error) {
	workspaceID, err := (WorkspaceResolver{FS: afero.NewOsFs()}).Resolve(cmd.Context(), workspace.WorkspaceID, true)
	if err != nil {
		return nil, "", "", err
	}
	workspace.WorkspaceID = workspaceID
	client, err := volcengineClientFromAuth(cmd)
	if err != nil {
		return nil, "", "", err
	}
	branchID := workspace.BranchID
	if branchID == "" {
		branch, err := client.ResolveDefaultBranch(cmd.Context(), workspace.WorkspaceID)
		if err != nil {
			return nil, "", "", err
		}
		branchID = branch.BranchID
	}
	return client, workspace.WorkspaceID, branchID, nil
}

func newAIGatewayStatusCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var token aiGatewayTokenOptions
	cmd := &cobra.Command{Use: "status", Short: "Show AI Gateway endpoint status", RunE: func(cmd *cobra.Command, args []string) error {
		_, gatewayURL, err := resolveAIGateway(cmd, ctx.FS, &workspace)
		if err != nil {
			return err
		}
		latency, err := (aigateway.Client{BaseURL: gatewayURL, Token: resolveAIGatewayToken(token)}).Check(cmd.Context())
		result := map[string]interface{}{"gateway_url": gatewayURL, "base_url": strings.TrimRight(gatewayURL, "/") + "/v1"}
		if err != nil {
			result["status"], result["error"] = "unavailable", err.Error()
		} else {
			result["status"], result["latency_ms"] = "ready", latency.Milliseconds()
		}
		if utils.OutputFormat.Value == utils.OutputPretty {
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(ctx.Output, "Gateway URL: %s\nStatus:      ready\nLatency:     %dms\n", gatewayURL, latency.Milliseconds())
			return err
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
	}}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	addAIGatewayTokenFlags(cmd, &token)
	return cmd
}

func newAIGatewayConfigCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var token aiGatewayTokenOptions
	var model string
	cmd := &cobra.Command{Use: "config", Short: "Show resolved AI Gateway configuration", RunE: func(cmd *cobra.Command, args []string) error {
		_, gatewayURL, err := resolveAIGateway(cmd, ctx.FS, &workspace)
		if err != nil {
			return err
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, map[string]interface{}{
			"workspace_id": workspace.WorkspaceID, "branch_id": workspace.BranchID,
			"gateway_url": gatewayURL, "base_url": strings.TrimRight(gatewayURL, "/") + "/v1",
			"model": firstNonEmpty(model, aigateway.DefaultModel), "token_source": aiGatewayTokenSource(token),
		})
	}}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	addAIGatewayTokenFlags(cmd, &token)
	cmd.Flags().StringVar(&model, "model", aigateway.DefaultModel, "Model name.")
	return cmd
}

func newAIGatewayAPIKeysCmd(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "api-keys", Short: "Manage AI Gateway API keys"}
	cmd.AddCommand(newAIGatewayAPIKeysListCmd(ctx))
	for _, action := range []string{"create", "revoke", "rotate"} {
		action := action
		cmd.AddCommand(&cobra.Command{Use: action, Short: action + " an AI Gateway API key", RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("AI Gateway API does not currently expose a confirmed %s API; only DescribeAPIKeys is available", action)
		}})
	}
	return cmd
}

func newAIGatewayAPIKeysListCmd(ctx RuntimeContext) *cobra.Command {
	var options aiGatewayAPIKeysOptions
	cmd := &cobra.Command{Use: "list", Short: "List AI Gateway API keys", RunE: func(cmd *cobra.Command, args []string) error {
		if options.ProjectName == "" {
			return errors.New("missing required flag: --project-name")
		}
		workspaceID, err := (WorkspaceResolver{FS: ctx.FS}).Resolve(cmd.Context(), options.WorkspaceID, true)
		if err != nil {
			return err
		}
		options.WorkspaceID = workspaceID
		return apiKeys.RunVolcengine(cmd.Context(), apiKeys.RunVolcengineParams{
			ProjectName: options.ProjectName, WorkspaceID: options.WorkspaceID, BranchID: options.BranchID, Fsys: ctx.FS, Reveal: options.Reveal,
		})
	}}
	addAIGatewayWorkspaceFlags(cmd, &options.aiGatewayWorkspaceOptions)
	cmd.Flags().StringVar(&options.ProjectName, "project-name", "", "Volcengine ProjectName.")
	cmd.Flags().BoolVar(&options.Reveal, "reveal", false, "Print complete API key values. Use only in a secure terminal.")
	return cmd
}

func newAIGatewayModelsCmd(ctx RuntimeContext) *cobra.Command {
	cmd := &cobra.Command{Use: "models", Short: "Manage AI Gateway models"}
	var workspace aiGatewayWorkspaceOptions
	var token aiGatewayTokenOptions
	list := &cobra.Command{Use: "list", Short: "List supported AI Gateway models", RunE: func(cmd *cobra.Command, args []string) error {
		_, gatewayURL, err := resolveAIGateway(cmd, ctx.FS, &workspace)
		if err != nil {
			return err
		}
		result, err := (aigateway.Client{BaseURL: gatewayURL, Token: resolveAIGatewayToken(token)}).Models(cmd.Context())
		if err != nil {
			return err
		}
		return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
	}}
	addAIGatewayWorkspaceFlags(list, &workspace)
	addAIGatewayTokenFlags(list, &token)
	cmd.AddCommand(list)
	return cmd
}

func newAIGatewayChatCmd(ctx RuntimeContext) *cobra.Command {
	var options aiGatewayChatOptions
	cmd := &cobra.Command{Use: "chat", Short: "Call AI Gateway chat APIs"}
	completions := &cobra.Command{Use: "completions", Short: "Create an OpenAI-compatible chat completion", RunE: func(cmd *cobra.Command, args []string) error {
		gatewayURL, err := gatewayURLForCall(cmd, ctx.FS, &options.aiGatewayWorkspaceOptions, options.aiGatewayTokenOptions)
		if err != nil {
			return err
		}
		if options.RequestFile != "" {
			data, err := afero.ReadFile(ctx.FS, options.RequestFile)
			if err != nil {
				return err
			}
			var request struct {
				Stream bool `json:"stream"`
			}
			if err := json.Unmarshal(data, &request); err != nil {
				return fmt.Errorf("parse --request: %w", err)
			}
			return (aigateway.Client{BaseURL: gatewayURL, Token: resolveAIGatewayToken(options.aiGatewayTokenOptions)}).ChatRaw(cmd.Context(), data, request.Stream, ctx.Output)
		}
		messages, err := aigateway.NewMessages(options.System, options.Message)
		if err != nil {
			return err
		}
		request := aigateway.ChatRequest{Model: options.Model, Messages: messages, Stream: options.Stream}
		if options.Temperature != 0 {
			request.Temperature = &options.Temperature
		}
		if options.MaxTokens > 0 {
			request.MaxTokens = &options.MaxTokens
		}
		if options.TopP != 0 {
			request.TopP = &options.TopP
		}
		return (aigateway.Client{BaseURL: gatewayURL, Token: resolveAIGatewayToken(options.aiGatewayTokenOptions)}).Chat(cmd.Context(), request, ctx.Output)
	}}
	addAIGatewayWorkspaceFlags(completions, &options.aiGatewayWorkspaceOptions)
	addAIGatewayTokenFlags(completions, &options.aiGatewayTokenOptions)
	completions.Flags().StringVar(&options.Model, "model", aigateway.DefaultModel, "Model name.")
	completions.Flags().StringVar(&options.Message, "message", "", "User message.")
	completions.Flags().StringVar(&options.System, "system", "", "Optional system message.")
	completions.Flags().StringVar(&options.RequestFile, "request", "", "Path to a complete JSON request.")
	completions.Flags().BoolVar(&options.Stream, "stream", false, "Stream response tokens.")
	completions.Flags().Float64Var(&options.Temperature, "temperature", 0, "Sampling temperature.")
	completions.Flags().IntVar(&options.MaxTokens, "max-tokens", 0, "Maximum output tokens.")
	completions.Flags().Float64Var(&options.TopP, "top-p", 0, "Nucleus sampling value.")
	cmd.AddCommand(completions)
	return cmd
}

func newAIGatewayTestCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var token aiGatewayTokenOptions
	cmd := &cobra.Command{Use: "test", Short: "Test AI Gateway connectivity", RunE: func(cmd *cobra.Command, args []string) error {
		gatewayURL, err := gatewayURLForCall(cmd, ctx.FS, &workspace, token)
		if err != nil {
			return err
		}
		start := time.Now()
		messages, _ := aigateway.NewMessages("", "Reply with OK.")
		err = (aigateway.Client{BaseURL: gatewayURL, Token: resolveAIGatewayToken(token)}).Chat(cmd.Context(), aigateway.ChatRequest{Model: aigateway.DefaultModel, Messages: messages}, ctx.Output)
		result := map[string]interface{}{"gateway_url": gatewayURL, "model": aigateway.DefaultModel, "latency_ms": time.Since(start).Milliseconds(), "status": "ready"}
		if err != nil {
			result["status"], result["error"] = "failed", err.Error()
		}
		if utils.OutputFormat.Value != utils.OutputPretty {
			_ = utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		}
		return err
	}}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	addAIGatewayTokenFlags(cmd, &token)
	return cmd
}

func newAIGatewayInitCmd(ctx RuntimeContext) *cobra.Command {
	var workspace aiGatewayWorkspaceOptions
	var model string
	cmd := &cobra.Command{Use: "init", Short: "Print AI Gateway environment configuration", RunE: func(cmd *cobra.Command, args []string) error {
		_, gatewayURL, err := resolveAIGateway(cmd, ctx.FS, &workspace)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(ctx.Output, "export SUPABASE_URL=%q\nexport AI_GATEWAY_BASE_URL=%q\nexport AI_GATEWAY_TOKEN=${AI_GATEWAY_TOKEN:-}\nexport AI_GATEWAY_MODEL=%q\n", gatewayURL, strings.TrimRight(gatewayURL, "/")+"/v1", model)
		return err
	}}
	addAIGatewayWorkspaceFlags(cmd, &workspace)
	cmd.Flags().StringVar(&model, "model", aigateway.DefaultModel, "Default model.")
	return cmd
}

func addAIGatewayWorkspaceFlags(cmd *cobra.Command, options *aiGatewayWorkspaceOptions) {
	cmd.Flags().StringVar(&options.WorkspaceID, "workspace-id", "", "Workspace ID.")
	cmd.Flags().StringVar(&options.WorkspaceID, "project-ref", "", "Workspace ID. Alias of --workspace-id.")
	cmd.Flags().StringVar(&options.BranchID, "branch-id", "", "Branch ID. Defaults to the workspace default branch.")
}

func addAIGatewayTokenFlags(cmd *cobra.Command, options *aiGatewayTokenOptions) {
	cmd.Flags().StringVar(&options.Token, "token", "", "AI Gateway bearer token.")
	cmd.Flags().StringVar(&options.TokenEnv, "token-env", "", "Environment variable containing the bearer token.")
}

func resolveAIGatewayToken(options aiGatewayTokenOptions) string {
	if strings.TrimSpace(options.Token) != "" {
		return options.Token
	}
	if options.TokenEnv != "" && os.Getenv(options.TokenEnv) != "" {
		return os.Getenv(options.TokenEnv)
	}
	for _, name := range []string{"AI_GATEWAY_TOKEN", "SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_ACCESS_TOKEN"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func aiGatewayTokenSource(options aiGatewayTokenOptions) string {
	if options.Token != "" {
		return "flag"
	}
	if options.TokenEnv != "" {
		return "token-env"
	}
	for _, name := range []string{"AI_GATEWAY_TOKEN", "SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_ACCESS_TOKEN"} {
		if os.Getenv(name) != "" {
			return name
		}
	}
	return "none"
}

func resolveAIGateway(cmd *cobra.Command, fsys afero.Fs, options *aiGatewayWorkspaceOptions) (volcengine.Config, string, error) {
	workspaceID, err := (WorkspaceResolver{FS: fsys}).Resolve(cmd.Context(), options.WorkspaceID, true)
	if err != nil {
		return volcengine.Config{}, "", err
	}
	options.WorkspaceID = workspaceID
	cfg, err := volcengineConfigFromAuth(cmd)
	if err != nil {
		return volcengine.Config{}, "", err
	}
	client, err := volcengineClientFromAuth(cmd)
	if err != nil {
		return volcengine.Config{}, "", err
	}
	branchID := options.BranchID
	if branchID == "" {
		branch, err := client.ResolveDefaultBranch(cmd.Context(), options.WorkspaceID)
		if err != nil {
			return volcengine.Config{}, "", err
		}
		branchID = branch.BranchID
		options.BranchID = branchID
	}
	endpoints, err := client.DescribeWorkspaceEndpoints(cmd.Context(), volcengine.DescribeWorkspaceEndpointsParams{WorkspaceID: options.WorkspaceID, BranchID: branchID})
	if err != nil {
		return volcengine.Config{}, "", err
	}
	for _, endpoint := range endpoints.Endpoints {
		for _, address := range endpoint.Addresses {
			if address.AddressDomain == "" {
				continue
			}
			domain := address.AddressDomain
			if !strings.Contains(domain, "://") {
				domain = "https://" + domain
			}
			if parsed, err := url.Parse(domain); err == nil && parsed.Hostname() != "" {
				return cfg, strings.TrimRight(domain, "/"), nil
			}
		}
	}
	return volcengine.Config{}, "", errors.New("no usable AI Gateway endpoint found for workspace branch")
}

func gatewayURLForCall(cmd *cobra.Command, fsys afero.Fs, workspace *aiGatewayWorkspaceOptions, token aiGatewayTokenOptions) (string, error) {
	_, gatewayURL, err := resolveAIGateway(cmd, fsys, workspace)
	if err != nil {
		return "", err
	}
	if resolveAIGatewayToken(token) == "" {
		return "", errors.New("missing AI Gateway token; use --token, --token-env, or AI_GATEWAY_TOKEN")
	}
	return gatewayURL, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
