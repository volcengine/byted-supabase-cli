// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/volcengine/volcengine-go-sdk/service/aidap"
)

func TestDescribeAPIKeysResponse(t *testing.T) {
	var response describeAPIKeysResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"Total": 1,
		"APIKeys": [{
			"CreateTime": "2026-09-01T02:28:19Z",
			"ExpireTime": "2026-09-02T11:40:05Z",
			"Key": "key-value",
			"Name": "DashboardLoginToken",
			"Type": "Dashboard"
		}]
	}`), &response))

	require.Equal(t, 1, response.Total)
	require.Len(t, response.APIKeys, 1)
	require.Equal(t, "DashboardLoginToken", response.APIKeys[0].Name)
	require.Equal(t, "2026-09-02T11:40:05Z", response.APIKeys[0].ExpireTime)
}

func TestAIGlobalQuotaRequestJSON(t *testing.T) {
	data, err := json.Marshal((&aidap.ModifyBaasAIGlobalQuotaInput{}).
		SetWorkspaceId("workspace-1").
		SetBranchId("branch-1").
		SetUserDefaultQuota((&aidap.UserDefaultQuotaForModifyBaasAIGlobalQuotaInput{}).
			SetDailyTokenLimit(100000)))
	require.NoError(t, err)
	require.JSONEq(t, `{"WorkspaceId":"workspace-1","BranchId":"branch-1","UserDefaultQuota":{"DailyTokenLimit":100000}}`, string(data))
}

func TestAIUserQuotaRequestJSON(t *testing.T) {
	data, err := json.Marshal((&aidap.ModifyBaasAIUserQuotasInput{}).
		SetWorkspaceId("workspace-1").
		SetBranchId("branch-1").
		SetUserQuotaItems([]*aidap.UserQuotaItemForModifyBaasAIUserQuotasInput{
			(&aidap.UserQuotaItemForModifyBaasAIUserQuotasInput{}).
				SetUserId("user-1").
				SetQuotaCustomized(true).
				SetCustomQuota((&aidap.CustomQuotaForModifyBaasAIUserQuotasInput{}).
					SetDailyTokenLimit(50000)),
		}))
	require.NoError(t, err)
	require.JSONEq(t, `{"WorkspaceId":"workspace-1","BranchId":"branch-1","UserQuotaItems":[{"UserId":"user-1","QuotaCustomized":true,"CustomQuota":{"DailyTokenLimit":50000}}]}`, string(data))
}
