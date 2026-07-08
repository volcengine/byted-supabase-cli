// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/volcengine/volcengine-go-sdk/service/aidap"
)

func TestMapWorkspaceFromCreateAgentPlanFields(t *testing.T) {
	workspace := mapWorkspaceFromCreate((&aidap.WorkspaceForCreateWorkspaceOutput{}).
		SetWorkspaceId("ws-1").
		SetIsAgentPlan(true).
		SetIsAgentPlanInstance(true).
		SetAgentPlanSeatId("seat-123"),
	)

	assert.Equal(t, "ws-1", workspace.WorkspaceID)
	assert.True(t, workspace.IsAgentPlan)
	assert.True(t, workspace.IsAgentPlanInstance)
	assert.Equal(t, "seat-123", workspace.AgentPlanSeatID)
}

func TestBuildAgentPlanSettings(t *testing.T) {
	assert.Nil(t, buildAgentPlanSettings(CreateWorkspaceParams{}))

	isAgentPlan := true
	personal := buildAgentPlanSettings(CreateWorkspaceParams{IsAgentPlan: &isAgentPlan})
	if assert.NotNil(t, personal) {
		assert.NotNil(t, personal.IsAgentPlan)
		assert.True(t, *personal.IsAgentPlan)
		assert.Nil(t, personal.AgentPlanSeatId)
	}

	enterprise := buildAgentPlanSettings(CreateWorkspaceParams{AgentPlanSeatID: "seat-123"})
	if assert.NotNil(t, enterprise) {
		assert.NotNil(t, enterprise.IsAgentPlan)
		assert.True(t, *enterprise.IsAgentPlan)
		assert.NotNil(t, enterprise.AgentPlanSeatId)
		assert.Equal(t, "seat-123", *enterprise.AgentPlanSeatId)
	}
}
