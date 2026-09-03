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
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
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
	cmd.AddCommand(
		newAIGatewayStatusCmd(ctx),
		newAIGatewayConfigCmd(ctx),
		newAIGatewayAPIKeysCmd(ctx),
		newAIGatewayModelsCmd(ctx),
		newAIGatewayChatCmd(ctx),
		newAIGatewayTestCmd(ctx),
		newAIGatewayInitCmd(ctx),
	)
	return cmd
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
		if options.WorkspaceID == "" {
			if err := loadOrPromptVolcengineProjectRef(cmd.Context(), ctx.FS, "Which workspace do you want to list AI Gateway API keys for?"); err != nil {
				return err
			}
			options.WorkspaceID = flags.ProjectRef
		} else if err := ensureVolcengineRegion(cmd.Context(), ctx.FS, false); err != nil {
			return err
		}
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
	if options.WorkspaceID == "" {
		if err := loadOrPromptVolcengineProjectRef(cmd.Context(), fsys, "Which workspace do you want to use for AI Gateway?"); err != nil {
			return volcengine.Config{}, "", err
		}
		options.WorkspaceID = flags.ProjectRef
	} else if err := ensureVolcengineRegion(cmd.Context(), fsys, false); err != nil {
		return volcengine.Config{}, "", err
	}
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return volcengine.Config{}, "", err
	}
	client := volcengine.NewClient(cfg)
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
