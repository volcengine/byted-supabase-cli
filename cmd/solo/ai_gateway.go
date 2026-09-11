package solo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/aigateway"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

type aiGatewayTokenOptions struct {
	Token    string
	TokenEnv string
}

type aiGatewayChatOptions struct {
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

type aiGatewayQuotaOptions struct {
	TokenLimit int64
	ImageLimit int64
}

func newAIGatewayCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{
		GroupID: "data-plane",
		Use:     "ai-gateway",
		Short:   "Manage Supabase AI Gateway",
	}
	command.AddCommand(
		newAIGatewayStatusCommand(ctx),
		newAIGatewayModelsCommand(ctx),
		newAIGatewayQuotaCommand(ctx),
		newAIGatewayUsageCommand(ctx),
		newAIGatewayChatCommand(ctx),
		newAIGatewayTestCommand(ctx),
	)
	return command
}

func newAIGatewayQuotaCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{Use: "quota", Short: "Manage AI Gateway quotas on the linked branch"}
	command.AddCommand(
		newAIGatewayQuotaGetCommand(ctx),
		newAIGatewayQuotaSetDefaultCommand(ctx),
		newAIGatewayQuotaUsersCommand(ctx),
	)
	return command
}

func soloAIGatewayQuotaClient() (*soloconfig.AIGatewayQuotaClient, error) {
	profile, err := soloconfig.LoadCurrentProfile()
	if err != nil {
		return nil, err
	}
	return soloconfig.NewAIGatewayQuotaClient(profile)
}

func newAIGatewayQuotaGetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use: "get", Short: "Show the default AI Gateway quota", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := soloAIGatewayQuotaClient()
			if err != nil {
				return err
			}
			result, err := client.GetDefaultQuota(cmd.Context())
			if err != nil {
				return err
			}
			if utils.OutputFormat.Value == utils.OutputPretty {
				_, err := fmt.Fprintf(ctx.Output, "Daily token limit: %d\nDaily image limit: %d\n", result.DailyTokenLimit, result.DailyImageLimit)
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
}

func newAIGatewayQuotaSetDefaultCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var options aiGatewayQuotaOptions
	command := &cobra.Command{
		Use: "set-default", Short: "Set the default AI Gateway quota", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("token-limit") && !cmd.Flags().Changed("image-limit") {
				return errors.New("at least one of --token-limit or --image-limit is required")
			}
			if err := confirmSoloWrite(cmd, "set the default AI Gateway quota"); err != nil {
				return err
			}
			client, err := soloAIGatewayQuotaClient()
			if err != nil {
				return err
			}
			var tokenLimit, imageLimit *int64
			if cmd.Flags().Changed("token-limit") {
				tokenLimit = &options.TokenLimit
			}
			if cmd.Flags().Changed("image-limit") {
				imageLimit = &options.ImageLimit
			}
			return client.SetDefaultQuota(cmd.Context(), soloconfig.AIGlobalQuotaPatch{
				DailyTokenLimit: tokenLimit,
				DailyImageLimit: imageLimit,
			})
		},
	}
	command.Flags().Int64Var(&options.TokenLimit, "token-limit", 0, "Daily token limit. Use -1 for unlimited.")
	command.Flags().Int64Var(&options.ImageLimit, "image-limit", 0, "Daily image generation limit. Use -1 for unlimited.")
	return command
}

func newAIGatewayQuotaUsersCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	command := &cobra.Command{Use: "users", Short: "Manage per-user AI Gateway quotas"}
	command.AddCommand(
		newAIGatewayQuotaUsersListCommand(ctx),
		newAIGatewayQuotaUsersGetCommand(ctx),
		newAIGatewayQuotaUsersSetCommand(ctx),
	)
	return command
}

func newAIGatewayQuotaUsersListCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var offset, limit int
	var search string
	command := &cobra.Command{
		Use: "list", Short: "List AI Gateway users, quotas, and usage", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if offset < 0 || limit < 1 || limit > 100 {
				return errors.New("--offset must be non-negative and --limit must be between 1 and 100")
			}
			client, err := soloAIGatewayQuotaClient()
			if err != nil {
				return err
			}
			result, err := client.ListUserSummaries(cmd.Context(), soloconfig.AIUserSummariesParams{
				Offset: offset, Limit: limit, Search: search,
			})
			if err != nil {
				return err
			}
			if utils.OutputFormat.Value == utils.OutputPretty {
				users := result.Users
				if users == nil {
					users = []soloconfig.AIUserSummary{}
				}
				return writeJSON(ctx, users)
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	command.Flags().IntVar(&offset, "offset", 0, "Number of users to skip.")
	command.Flags().IntVar(&limit, "limit", 100, "Maximum number of users to return (1-100).")
	command.Flags().StringVar(&search, "search", "", "Search users by email, name, or phone.")
	return command
}

func newAIGatewayQuotaUsersGetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var userID string
	command := &cobra.Command{
		Use: "get", Short: "Show one AI Gateway user's quota and usage", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(userID) == "" {
				return errors.New("missing required flag: --user-id")
			}
			client, err := soloAIGatewayQuotaClient()
			if err != nil {
				return err
			}
			for offset := 0; ; offset += 100 {
				result, err := client.ListUserSummaries(cmd.Context(), soloconfig.AIUserSummariesParams{Offset: offset, Limit: 100})
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
	command.Flags().StringVar(&userID, "user-id", "", "User ID.")
	return command
}

func newAIGatewayQuotaUsersSetCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var userID string
	var tokenLimit, imageLimit int64
	var reset bool
	command := &cobra.Command{
		Use: "set", Short: "Set or reset one AI Gateway user's quota", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(userID) == "" {
				return errors.New("missing required flag: --user-id")
			}
			if !reset && !cmd.Flags().Changed("token-limit") && !cmd.Flags().Changed("image-limit") {
				return errors.New("at least one quota flag or --reset is required")
			}
			if err := confirmSoloWrite(cmd, "update an AI Gateway user's quota"); err != nil {
				return err
			}
			client, err := soloAIGatewayQuotaClient()
			if err != nil {
				return err
			}
			customized := !reset
			update := soloconfig.AIUserQuotaUpdate{
				UserID: userID, QuotaCustomized: &customized,
			}
			if customized {
				var tokenLimitPtr, imageLimitPtr *int64
				if cmd.Flags().Changed("token-limit") {
					tokenLimitPtr = &tokenLimit
				}
				if cmd.Flags().Changed("image-limit") {
					imageLimitPtr = &imageLimit
				}
				update.CustomQuota = &soloconfig.AIGlobalQuotaPatch{
					DailyTokenLimit: tokenLimitPtr, DailyImageLimit: imageLimitPtr,
				}
			}
			return client.UpdateUsers(cmd.Context(), soloconfig.AIUserQuotaUpdates{Items: []soloconfig.AIUserQuotaUpdate{update}})
		},
	}
	command.Flags().StringVar(&userID, "user-id", "", "User ID.")
	command.Flags().Int64Var(&tokenLimit, "token-limit", 0, "Daily token limit. Use -1 for unlimited.")
	command.Flags().Int64Var(&imageLimit, "image-limit", 0, "Daily image generation limit. Use -1 for unlimited.")
	command.Flags().BoolVar(&reset, "reset", false, "Restore the user to the default quota.")
	return command
}

func newAIGatewayUsageCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	return &cobra.Command{
		Use: "usage", Short: "Show aggregated AI Gateway usage on the linked branch", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := soloAIGatewayQuotaClient()
			if err != nil {
				return err
			}
			usage := soloconfig.AIUsage{}
			for offset := 0; ; offset += 100 {
				result, err := client.ListUserSummaries(cmd.Context(), soloconfig.AIUserSummariesParams{Offset: offset, Limit: 100})
				if err != nil {
					return err
				}
				usage.UserCount = result.Total
				for _, user := range result.Users {
					if user.Usage == nil {
						continue
					}
					usage.TodayTokens += user.Usage.LastDailyTokens
					usage.TotalTokens += user.Usage.TotalTokens
					usage.TodayImages += user.Usage.LastDailyImages
					usage.TotalImages += user.Usage.TotalImages
				}
				if len(result.Users) == 0 || offset+len(result.Users) >= result.Total {
					break
				}
			}
			if utils.OutputFormat.Value == utils.OutputPretty {
				_, err := fmt.Fprintf(ctx.Output, "Users: %d\nTokens today: %d\nTokens total: %d\nImages today: %d\nImages total: %d\n",
					usage.UserCount, usage.TodayTokens, usage.TotalTokens, usage.TodayImages, usage.TotalImages)
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, usage)
		},
	}
}

func usageTokensToday(user soloconfig.AIUserSummary) int64 {
	if user.Usage == nil {
		return 0
	}
	return user.Usage.LastDailyTokens
}

func usageTokensTotal(user soloconfig.AIUserSummary) int64 {
	if user.Usage == nil {
		return 0
	}
	return user.Usage.TotalTokens
}

func usageImagesToday(user soloconfig.AIUserSummary) int64 {
	if user.Usage == nil {
		return 0
	}
	return user.Usage.LastDailyImages
}

func usageImagesTotal(user soloconfig.AIUserSummary) int64 {
	if user.Usage == nil {
		return 0
	}
	return user.Usage.TotalImages
}

func newAIGatewayStatusCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var token aiGatewayTokenOptions
	return &cobra.Command{
		Use:   "status",
		Short: "Show AI Gateway endpoint status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gatewayURL, err := soloAIGatewayURL()
			if err != nil {
				return err
			}
			latency, checkErr := (aigateway.Client{
				BaseURL: gatewayURL,
				Token:   resolveSoloAIGatewayToken(token),
			}).Check(cmd.Context())
			result := map[string]interface{}{
				"gateway_url": gatewayURL,
				"base_url":    strings.TrimRight(gatewayURL, "/") + "/v1",
			}
			if checkErr != nil {
				result["status"], result["error"] = "unavailable", checkErr.Error()
			} else {
				result["status"], result["latency_ms"] = "ready", latency.Milliseconds()
			}
			if utils.OutputFormat.Value == utils.OutputPretty {
				if checkErr != nil {
					return checkErr
				}
				_, err = fmt.Fprintf(ctx.Output, "Gateway URL: %s\nStatus:      ready\nLatency:     %dms\n", gatewayURL, latency.Milliseconds())
				return err
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
}

func newAIGatewayModelsCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var token aiGatewayTokenOptions
	command := &cobra.Command{Use: "models", Short: "Manage AI Gateway models"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List supported AI Gateway models",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gatewayURL, err := soloAIGatewayURL()
			if err != nil {
				return err
			}
			result, err := (aigateway.Client{
				BaseURL: gatewayURL,
				Token:   resolveSoloAIGatewayToken(token),
			}).Models(cmd.Context())
			if err != nil {
				return err
			}
			if utils.OutputFormat.Value == utils.OutputPretty {
				for _, model := range result.Data {
					if _, err := fmt.Fprintln(ctx.Output, model.ID); err != nil {
						return err
					}
				}
				return nil
			}
			return utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
		},
	}
	addSoloAIGatewayTokenFlags(list, &token)
	command.AddCommand(list)
	return command
}

func newAIGatewayChatCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var options aiGatewayChatOptions
	command := &cobra.Command{Use: "chat", Short: "Call AI Gateway chat APIs"}
	completions := &cobra.Command{
		Use:   "completions",
		Short: "Create an OpenAI-compatible chat completion",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gatewayURL, err := soloAIGatewayURLForCall(options.aiGatewayTokenOptions)
			if err != nil {
				return err
			}
			client := aigateway.Client{
				BaseURL: gatewayURL,
				Token:   resolveSoloAIGatewayToken(options.aiGatewayTokenOptions),
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
				return client.ChatRaw(cmd.Context(), data, request.Stream, ctx.Output)
			}
			messages, err := aigateway.NewMessages(options.System, options.Message)
			if err != nil {
				return err
			}
			request := aigateway.ChatRequest{
				Model: options.Model, Messages: messages, Stream: options.Stream,
			}
			if options.Temperature != 0 {
				request.Temperature = &options.Temperature
			}
			if options.MaxTokens > 0 {
				request.MaxTokens = &options.MaxTokens
			}
			if options.TopP != 0 {
				request.TopP = &options.TopP
			}
			return client.Chat(cmd.Context(), request, ctx.Output)
		},
	}
	addSoloAIGatewayTokenFlags(completions, &options.aiGatewayTokenOptions)
	completions.Flags().StringVar(&options.Model, "model", aigateway.DefaultModel, "Model name.")
	completions.Flags().StringVar(&options.Message, "message", "", "User message.")
	completions.Flags().StringVar(&options.System, "system", "", "Optional system message.")
	completions.Flags().StringVar(&options.RequestFile, "request", "", "Path to a complete JSON request.")
	completions.Flags().BoolVar(&options.Stream, "stream", false, "Stream response tokens.")
	completions.Flags().Float64Var(&options.Temperature, "temperature", 0, "Sampling temperature.")
	completions.Flags().IntVar(&options.MaxTokens, "max-tokens", 0, "Maximum output tokens.")
	completions.Flags().Float64Var(&options.TopP, "top-p", 0, "Nucleus sampling value.")
	command.AddCommand(completions)
	return command
}

func newAIGatewayTestCommand(ctx commandtree.RuntimeContext) *cobra.Command {
	var token aiGatewayTokenOptions
	return &cobra.Command{
		Use:   "test",
		Short: "Test AI Gateway connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gatewayURL, err := soloAIGatewayURLForCall(token)
			if err != nil {
				return err
			}
			start := time.Now()
			messages, _ := aigateway.NewMessages("", "Reply with OK.")
			err = (aigateway.Client{
				BaseURL: gatewayURL,
				Token:   resolveSoloAIGatewayToken(token),
			}).Chat(cmd.Context(), aigateway.ChatRequest{
				Model: aigateway.DefaultModel, Messages: messages,
			}, ctx.Output)
			result := map[string]interface{}{
				"gateway_url": gatewayURL,
				"model":       aigateway.DefaultModel,
				"latency_ms":  time.Since(start).Milliseconds(),
				"status":      "ready",
			}
			if err != nil {
				result["status"], result["error"] = "failed", err.Error()
			}
			if utils.OutputFormat.Value != utils.OutputPretty {
				_ = utils.EncodeOutput(utils.OutputFormat.Value, ctx.Output, result)
			}
			return err
		},
	}
}

func addSoloAIGatewayTokenFlags(command *cobra.Command, options *aiGatewayTokenOptions) {
	command.Flags().StringVar(&options.Token, "token", "", "AI Gateway bearer token.")
	command.Flags().StringVar(&options.TokenEnv, "token-env", "", "Environment variable containing the bearer token.")
}

func soloAIGatewayURL() (string, error) {
	profile, err := soloconfig.LoadCurrentProfile()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(profile.SupabaseURL) == "" {
		return "", errors.New("missing Supabase URL in the current linked-branch profile")
	}
	return strings.TrimRight(profile.SupabaseURL, "/"), nil
}

func soloAIGatewayURLForCall(options aiGatewayTokenOptions) (string, error) {
	gatewayURL, err := soloAIGatewayURL()
	if err != nil {
		return "", err
	}
	if resolveSoloAIGatewayToken(options) == "" {
		return "", errors.New("missing AI Gateway token; use --token, --token-env, AI_GATEWAY_TOKEN, or the linked branch Service Role Key")
	}
	return gatewayURL, nil
}

func resolveSoloAIGatewayToken(options aiGatewayTokenOptions) string {
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
	if profile, err := soloconfig.LoadCurrentProfile(); err == nil {
		return profile.ServiceRoleKey
	}
	return ""
}

func soloAIGatewayTokenSource(options aiGatewayTokenOptions) string {
	if options.Token != "" {
		return "flag"
	}
	if options.TokenEnv != "" && os.Getenv(options.TokenEnv) != "" {
		return "token-env"
	}
	for _, name := range []string{"AI_GATEWAY_TOKEN", "SUPABASE_SERVICE_ROLE_KEY", "SUPABASE_ACCESS_TOKEN"} {
		if os.Getenv(name) != "" {
			return name
		}
	}
	if profile, err := soloconfig.LoadCurrentProfile(); err == nil && profile.ServiceRoleKey != "" {
		return "solo-service-role-key"
	}
	return "none"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
