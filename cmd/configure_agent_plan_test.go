// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAgentPlanProfileUpdate(t *testing.T) {
	profile, changed, err := resolveAgentPlanProfileUpdate(false, false, false, "")
	require.NoError(t, err)
	assert.False(t, changed)

	profile, changed, err = resolveAgentPlanProfileUpdate(true, true, false, "")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, profile.IsAgentPlan)
	assert.Empty(t, profile.AgentPlanSeatID)

	profile, changed, err = resolveAgentPlanProfileUpdate(false, false, true, " seat-123 ")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, profile.IsAgentPlan)
	assert.Equal(t, "seat-123", profile.AgentPlanSeatID)

	profile, changed, err = resolveAgentPlanProfileUpdate(true, false, false, "")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, profile.IsAgentPlan)
	assert.Empty(t, profile.AgentPlanSeatID)

	_, _, err = resolveAgentPlanProfileUpdate(true, false, true, "seat-123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--is-agent-plan=false")
}
