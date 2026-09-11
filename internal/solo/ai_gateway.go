package solo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	linkstate "github.com/volcengine/byted-supabase-cli/internal/link"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type AIGatewayQuotaClient struct {
	baseURL        string
	serviceRoleKey string
	httpClient     *http.Client
}

type AIGlobalQuota struct {
	DailyTokenLimit int64 `json:"daily_token_limit"`
	DailyImageLimit int64 `json:"daily_image_limit"`
}

func (q *AIGlobalQuota) UnmarshalJSON(data []byte) error {
	type quotaAlias AIGlobalQuota
	var value struct {
		*quotaAlias
		DailyTokenLimitPascal *int64 `json:"DailyTokenLimit"`
		DailyImageLimitPascal *int64 `json:"DailyImageLimit"`
	}
	value.quotaAlias = (*quotaAlias)(q)
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.DailyTokenLimitPascal != nil {
		q.DailyTokenLimit = *value.DailyTokenLimitPascal
	}
	if value.DailyImageLimitPascal != nil {
		q.DailyImageLimit = *value.DailyImageLimitPascal
	}
	return nil
}

type AIGlobalQuotaPatch struct {
	DailyTokenLimit *int64 `json:"daily_token_limit,omitempty"`
	DailyImageLimit *int64 `json:"daily_image_limit,omitempty"`
}

type aiGlobalQuotaResponse struct {
	UserDefaultQuota *AIGlobalQuota `json:"user_default_quota"`
	DailyTokenLimit  *int64         `json:"daily_token_limit"`
	DailyImageLimit  *int64         `json:"daily_image_limit"`
}

type AIUserSummary struct {
	UserID          string         `json:"user_id"`
	UserName        string         `json:"user_name,omitempty"`
	Email           string         `json:"email,omitempty"`
	Phone           string         `json:"phone,omitempty"`
	CreateTime      string         `json:"create_time,omitempty"`
	LastSignInTime  string         `json:"last_sign_in_time,omitempty"`
	QuotaCustomized bool           `json:"quota_customized"`
	CustomQuota     *AIGlobalQuota `json:"custom_quota,omitempty"`
	Usage           *AIUserUsage   `json:"usage,omitempty"`
}

func (u *AIUserSummary) UnmarshalJSON(data []byte) error {
	type summaryAlias AIUserSummary
	var value struct {
		*summaryAlias
		UserIDPascal          string         `json:"UserId"`
		UserNamePascal        string         `json:"UserName"`
		EmailPascal           string         `json:"Email"`
		PhonePascal           string         `json:"Phone"`
		CreateTimePascal      string         `json:"CreateTime"`
		LastSignInTimePascal  string         `json:"LastSignInTime"`
		QuotaCustomizedPascal *bool          `json:"QuotaCustomized"`
		CustomQuotaPascal     *AIGlobalQuota `json:"CustomQuota"`
		UsagePascal           *AIUserUsage   `json:"Usage"`
	}
	value.summaryAlias = (*summaryAlias)(u)
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.UserIDPascal != "" {
		u.UserID = value.UserIDPascal
	}
	if value.UserNamePascal != "" {
		u.UserName = value.UserNamePascal
	}
	if value.EmailPascal != "" {
		u.Email = value.EmailPascal
	}
	if value.PhonePascal != "" {
		u.Phone = value.PhonePascal
	}
	if value.CreateTimePascal != "" {
		u.CreateTime = value.CreateTimePascal
	}
	if value.LastSignInTimePascal != "" {
		u.LastSignInTime = value.LastSignInTimePascal
	}
	if value.QuotaCustomizedPascal != nil {
		u.QuotaCustomized = *value.QuotaCustomizedPascal
	}
	if value.CustomQuotaPascal != nil {
		u.CustomQuota = value.CustomQuotaPascal
	}
	if value.UsagePascal != nil {
		u.Usage = value.UsagePascal
	}
	return nil
}

type AIUserUsage struct {
	LastDailyTokens int64 `json:"last_daily_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
	LastDailyImages int64 `json:"last_daily_images"`
	TotalImages     int64 `json:"total_images"`
}

func (u *AIUserUsage) UnmarshalJSON(data []byte) error {
	type usageAlias AIUserUsage
	var value struct {
		*usageAlias
		LastDailyTokensPascal *int64 `json:"LastDailyTokens"`
		TotalTokensPascal     *int64 `json:"TotalTokens"`
		LastDailyImagesPascal *int64 `json:"LastDailyImages"`
		TotalImagesPascal     *int64 `json:"TotalImages"`
	}
	value.usageAlias = (*usageAlias)(u)
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.LastDailyTokensPascal != nil {
		u.LastDailyTokens = *value.LastDailyTokensPascal
	}
	if value.TotalTokensPascal != nil {
		u.TotalTokens = *value.TotalTokensPascal
	}
	if value.LastDailyImagesPascal != nil {
		u.LastDailyImages = *value.LastDailyImagesPascal
	}
	if value.TotalImagesPascal != nil {
		u.TotalImages = *value.TotalImagesPascal
	}
	return nil
}

type AIUserSummariesParams struct {
	Offset int
	Limit  int
	Search string
}

type AIUserSummaries struct {
	Total int             `json:"total"`
	Users []AIUserSummary `json:"users"`
}

type aiUserSummariesResponse struct {
	Total                  int             `json:"total"`
	Users                  []AIUserSummary `json:"users"`
	UserSummaryItems       []AIUserSummary `json:"user_summary_items"`
	TotalPascal            int             `json:"Total"`
	UserSummaryItemsPascal []AIUserSummary `json:"UserSummaryItems"`
	Items                  []AIUserSummary `json:"items"`
	ItemsPascal            []AIUserSummary `json:"Items"`
	Data                   []AIUserSummary `json:"data"`
	DataPascal             []AIUserSummary `json:"Data"`
	UserSummaries          []AIUserSummary `json:"user_summaries"`
	UserSummariesPascal    []AIUserSummary `json:"UserSummaries"`
}

type AIUsage struct {
	UserCount   int   `json:"user_count"`
	TodayTokens int64 `json:"today_tokens"`
	TotalTokens int64 `json:"total_tokens"`
	TodayImages int64 `json:"today_images"`
	TotalImages int64 `json:"total_images"`
}

type AIUserQuotaUpdate struct {
	UserID          string              `json:"user_id"`
	QuotaCustomized *bool               `json:"quota_customized,omitempty"`
	CustomQuota     *AIGlobalQuotaPatch `json:"custom_quota,omitempty"`
	Enabled         *bool               `json:"enabled,omitempty"`
}

type AIUserQuotaUpdates struct {
	Items []AIUserQuotaUpdate `json:"items"`
}

func NewAIGatewayQuotaClient(profile Profile) (*AIGatewayQuotaClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("missing Supabase URL in the current linked-branch profile")
	}
	if strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return nil, fmt.Errorf("missing service role key in the current linked-branch profile")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	return &AIGatewayQuotaClient{
		baseURL:        baseURL,
		serviceRoleKey: strings.TrimSpace(profile.ServiceRoleKey),
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *AIGatewayQuotaClient) GetDefaultQuota(ctx context.Context) (AIGlobalQuota, error) {
	var response aiGlobalQuotaResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/v1/aigateway/quota/default", nil, &response); err != nil {
		return AIGlobalQuota{}, err
	}
	if response.UserDefaultQuota != nil {
		return *response.UserDefaultQuota, nil
	}
	result := AIGlobalQuota{}
	if response.DailyTokenLimit != nil {
		result.DailyTokenLimit = *response.DailyTokenLimit
	}
	if response.DailyImageLimit != nil {
		result.DailyImageLimit = *response.DailyImageLimit
	}
	return result, nil
}

func (c *AIGatewayQuotaClient) SetDefaultQuota(ctx context.Context, quota AIGlobalQuotaPatch) error {
	return c.requestJSON(ctx, http.MethodPut, "/v1/aigateway/quota/default", struct {
		UserDefaultQuota AIGlobalQuotaPatch `json:"user_default_quota"`
	}{UserDefaultQuota: quota}, nil)
}

func (c *AIGatewayQuotaClient) ListUserSummaries(ctx context.Context, params AIUserSummariesParams) (AIUserSummaries, error) {
	query := url.Values{}
	query.Set("offset", strconv.Itoa(params.Offset))
	query.Set("limit", strconv.Itoa(params.Limit))
	if strings.TrimSpace(params.Search) != "" {
		query.Set("search", params.Search)
	}
	var response aiUserSummariesResponse
	request := struct {
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
		Search string `json:"search,omitempty"`
	}{Offset: params.Offset, Limit: params.Limit, Search: params.Search}
	if err := c.requestJSON(ctx, http.MethodPost, "/v1/aigateway/quota/users/summaries", request, &response); err != nil {
		if isNotFoundError(err) {
			if result, fallbackErr := listUserSummariesFromManagement(ctx, params); fallbackErr == nil {
				return result, nil
			}
		}
		return AIUserSummaries{}, err
	}
	total := response.Total
	if total == 0 {
		total = response.TotalPascal
	}
	users := response.Users
	if len(users) == 0 {
		users = response.UserSummaryItems
	}
	if len(users) == 0 {
		users = response.UserSummaryItemsPascal
	}
	if len(users) == 0 {
		users = response.Items
	}
	if len(users) == 0 {
		users = response.ItemsPascal
	}
	if len(users) == 0 {
		users = response.Data
	}
	if len(users) == 0 {
		users = response.DataPascal
	}
	if len(users) == 0 {
		users = response.UserSummaries
	}
	if len(users) == 0 {
		users = response.UserSummariesPascal
	}
	if len(users) == 0 && total > 0 {
		if result, fallbackErr := listUserSummariesFromManagement(ctx, params); fallbackErr == nil {
			return result, nil
		} else {
			return AIUserSummaries{}, fmt.Errorf("AI Gateway returned %d users but no user details; management fallback failed: %w", total, fallbackErr)
		}
	}
	return AIUserSummaries{Total: total, Users: users}, nil
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "(404 Not Found)")
}

func listUserSummariesFromManagement(ctx context.Context, params AIUserSummariesParams) (AIUserSummaries, error) {
	state, err := linkstate.LoadStateAtDir(".")
	if err != nil || state.WorkspaceID == "" || state.Type != linkstate.LinkTypeSolo {
		return AIUserSummaries{}, fmt.Errorf("solo link state does not contain a workspace")
	}
	configSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return AIUserSummaries{}, err
	}
	for _, region := range configSet.Regions {
		client := volcengine.NewClient(configSet.ConfigForRegion(region))
		branchID := state.BranchID
		if branchID == "" {
			branch, resolveErr := client.ResolveDefaultBranch(ctx, state.WorkspaceID)
			if resolveErr != nil {
				continue
			}
			branchID = branch.BranchID
		}
		result, callErr := client.DescribeAIUserSummaries(ctx, volcengine.DescribeAIUserSummariesParams{
			WorkspaceID: state.WorkspaceID,
			BranchID:    branchID,
			Limit:       params.Limit,
			Offset:      params.Offset,
		})
		if callErr != nil {
			continue
		}
		users := make([]AIUserSummary, 0, len(result.Users))
		for _, user := range result.Users {
			summary := AIUserSummary{
				UserID: user.UserID, UserName: user.UserName, Email: user.Email,
				Phone: user.Phone, CreateTime: user.CreateTime, LastSignInTime: user.LastSignInTime,
				QuotaCustomized: user.QuotaCustomized,
				Usage: &AIUserUsage{
					LastDailyTokens: user.LastDailyTokens, TotalTokens: user.TotalTokens,
					LastDailyImages: user.LastDailyImages, TotalImages: user.TotalImages,
				},
			}
			if user.QuotaCustomized {
				summary.CustomQuota = &AIGlobalQuota{
					DailyTokenLimit: user.DailyTokenLimit,
					DailyImageLimit: user.DailyImageLimit,
				}
			}
			users = append(users, summary)
		}
		return AIUserSummaries{Total: result.Total, Users: users}, nil
	}
	return AIUserSummaries{}, fmt.Errorf("failed to list AI Gateway users through management API")
}

func (c *AIGatewayQuotaClient) UpdateUsers(ctx context.Context, updates AIUserQuotaUpdates) error {
	return c.requestJSON(ctx, http.MethodPut, "/v1/aigateway/quota/users", updates, nil)
}

func (c *AIGatewayQuotaClient) requestJSON(ctx context.Context, method, path string, input, output interface{}) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode AI Gateway quota request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create AI Gateway quota request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("AI Gateway quota request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("AI Gateway quota request failed (%s): %s", response.Status, message)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode AI Gateway quota response: %w", err)
	}
	return nil
}
