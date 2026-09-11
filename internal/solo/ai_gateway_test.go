package solo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayQuotaClientUsesDataPlaneRoutesAndServiceRole(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer service-role-key", request.Header.Get("Authorization"))
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		requests = append(requests, request.Method+" "+request.URL.Path+" "+string(body))

		switch request.URL.Path {
		case "/v1/aigateway/quota/default":
			if request.Method == http.MethodGet {
				_, _ = io.WriteString(writer, `{"daily_token_limit":100000,"daily_image_limit":50}`)
				return
			}
			_, _ = io.WriteString(writer, `{}`)
		case "/v1/aigateway/quota/users/summaries":
			_, _ = io.WriteString(writer, `{"total":1,"users":[{"user_id":"user-1","quota_customized":true,"custom_quota":{"daily_token_limit":200000},"usage":{"last_daily_tokens":12,"total_tokens":100,"last_daily_images":1,"total_images":2}}]}`)
		case "/v1/aigateway/quota/users":
			_, _ = io.WriteString(writer, `{}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client, err := NewAIGatewayQuotaClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	quota, err := client.GetDefaultQuota(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(100000), quota.DailyTokenLimit)
	require.Equal(t, int64(50), quota.DailyImageLimit)

	tokenLimit := int64(100000)
	imageLimit := int64(50)
	require.NoError(t, client.SetDefaultQuota(context.Background(), AIGlobalQuotaPatch{
		DailyTokenLimit: &tokenLimit,
		DailyImageLimit: &imageLimit,
	}))

	summaries, err := client.ListUserSummaries(context.Background(), AIUserSummariesParams{
		Offset: 10,
		Limit:  20,
		Search: "@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, 1, summaries.Total)
	require.Equal(t, "user-1", summaries.Users[0].UserID)

	customized := true
	require.NoError(t, client.UpdateUsers(context.Background(), AIUserQuotaUpdates{
		Items: []AIUserQuotaUpdate{{
			UserID:          "user-1",
			QuotaCustomized: &customized,
			CustomQuota:     &AIGlobalQuotaPatch{DailyTokenLimit: &tokenLimit},
		}},
	}))

	require.Len(t, requests, 4)
	require.Contains(t, requests[0], "GET /v1/aigateway/quota/default")
	require.Contains(t, requests[1], "PUT /v1/aigateway/quota/default")
	require.Contains(t, requests[2], "POST /v1/aigateway/quota/users/summaries")
	require.Contains(t, requests[2], `"offset":10`)
	require.Contains(t, requests[2], `"limit":20`)
	require.Contains(t, requests[2], `"search":"@example.com"`)
	require.Contains(t, requests[3], "PUT /v1/aigateway/quota/users")
}

func TestAIGatewayQuotaClientParsesPascalCaseUserSummaries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/v1/aigateway/quota/users/summaries", request.URL.Path)
		_, _ = io.WriteString(writer, `{"Total":2,"UserSummaryItems":[{"UserId":"user-1","UserName":"Alice","Email":"alice@example.com","QuotaCustomized":true,"CustomQuota":{"DailyTokenLimit":200000,"DailyImageLimit":20},"Usage":{"LastDailyTokens":12,"TotalTokens":100,"LastDailyImages":1,"TotalImages":2}},{"UserId":"user-2","Email":"bob@example.com","Usage":{"LastDailyTokens":3,"TotalTokens":30,"LastDailyImages":0,"TotalImages":1}}]}`)
	}))
	defer server.Close()

	client, err := NewAIGatewayQuotaClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	result, err := client.ListUserSummaries(context.Background(), AIUserSummariesParams{Limit: 100})
	require.NoError(t, err)
	require.Equal(t, 2, result.Total)
	require.Len(t, result.Users, 2)
	require.Equal(t, "user-1", result.Users[0].UserID)
	require.Equal(t, "alice@example.com", result.Users[0].Email)
	require.True(t, result.Users[0].QuotaCustomized)
	require.Equal(t, int64(200000), result.Users[0].CustomQuota.DailyTokenLimit)
	require.Equal(t, int64(12), result.Users[0].Usage.LastDailyTokens)
	require.Equal(t, int64(100), result.Users[0].Usage.TotalTokens)
	require.Equal(t, "user-2", result.Users[1].UserID)
	require.Equal(t, int64(30), result.Users[1].Usage.TotalTokens)
}

func TestIsNotFoundError(t *testing.T) {
	require.True(t, isNotFoundError(fmt.Errorf("AI Gateway quota request failed (404 Not Found): Not Found")))
	require.False(t, isNotFoundError(fmt.Errorf("AI Gateway quota request failed (403 Forbidden): Forbidden")))
}

func TestAIUserSummariesUsesNonNilEmptyUsers(t *testing.T) {
	result := AIUserSummaries{Total: 2, Users: nil}
	if result.Users == nil {
		result.Users = []AIUserSummary{}
	}
	data, err := json.Marshal(result.Users)
	require.NoError(t, err)
	require.Equal(t, "[]", string(data))
}

func TestAIGatewayQuotaClientRejectsMissingServiceRoleKey(t *testing.T) {
	_, err := NewAIGatewayQuotaClient(Profile{SupabaseURL: "https://example.com"})
	require.ErrorContains(t, err, "missing service role key")
}

func TestAIGatewayQuotaUpdateResetPayload(t *testing.T) {
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &payload))
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()

	client, err := NewAIGatewayQuotaClient(Profile{
		SupabaseURL:    server.URL,
		ServiceRoleKey: "service-role-key",
	})
	require.NoError(t, err)

	customized := false
	require.NoError(t, client.UpdateUsers(context.Background(), AIUserQuotaUpdates{
		Items: []AIUserQuotaUpdate{{
			UserID:          "service_role:system",
			QuotaCustomized: &customized,
		}},
	}))
	require.Equal(t, false, payload["items"].([]interface{})[0].(map[string]interface{})["quota_customized"])
	require.NotContains(t, strings.TrimSpace(string(mustJSON(payload))), "custom_quota")
}

func mustJSON(value interface{}) []byte {
	data, _ := json.Marshal(value)
	return data
}
