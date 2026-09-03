// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
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
