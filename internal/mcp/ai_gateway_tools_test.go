// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayToolsListed(t *testing.T) {
	names := listToolNames(t, connect(t, Options{Features: []string{featureAIGateway}}))
	for _, name := range []string{
		"ai_gateway_status",
		"ai_gateway_models",
		"ai_gateway_chat",
		"ai_gateway_get_quota",
		"ai_gateway_set_quota",
		"ai_gateway_list_users",
		"ai_gateway_get_user",
		"ai_gateway_set_user_quota",
		"ai_gateway_usage",
	} {
		assert.True(t, names[name], "expected %s to be listed", name)
	}
}

func TestAIGatewayFeatureGating(t *testing.T) {
	names := listToolNames(t, connect(t, Options{Features: []string{featureDatabase}}))
	assert.False(t, names["ai_gateway_models"])
	assert.True(t, names["execute_sql"])
}

func TestAIGatewayReadOnlyHidesCallsAndWrites(t *testing.T) {
	names := listToolNames(t, connect(t, Options{ReadOnly: true, Features: []string{featureAIGateway}}))
	assert.True(t, names["ai_gateway_status"])
	assert.True(t, names["ai_gateway_models"])
	assert.True(t, names["ai_gateway_get_quota"])
	assert.True(t, names["ai_gateway_list_users"])
	assert.True(t, names["ai_gateway_get_user"])
	assert.True(t, names["ai_gateway_usage"])
	assert.False(t, names["ai_gateway_chat"])
	assert.False(t, names["ai_gateway_set_quota"])
	assert.False(t, names["ai_gateway_set_user_quota"])
}

func TestAIGatewayChatValidatesModelBeforeAuthentication(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAIGateway}})
	_, err := aiGatewayChat(context.Background(), &p, aiGatewayCallInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model is required")
}

func TestAIGatewayQuotaValidatesLimitsBeforeAuthentication(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAIGateway}})
	_, err := aiGatewaySetQuota(context.Background(), &p, aiGatewayQuotaInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one")
}

func TestAIGatewayUserQuotaValidation(t *testing.T) {
	p := newPolicy(Options{Features: []string{featureAIGateway}})
	value := int64(1)
	_, err := aiGatewaySetUserQuota(context.Background(), &p, aiGatewaySetUserInput{
		UserID:          "user-1",
		Reset:           true,
		DailyTokenLimit: &value,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reset cannot be combined")
}
