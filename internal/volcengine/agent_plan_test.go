// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"testing"

	"github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool { return &v }

// fakeAgentPlanQuerier is a test double for agentPlanQuerier. It records the seat IDs
// passed to GetSeatAFPUsage so tests can assert the request was trimmed correctly.
type fakeAgentPlanQuerier struct {
	personal    PersonalPlanResult
	personalErr error
	usages      []SeatAFPUsage
	usagesErr   error

	gotPlan    string
	gotSeatIDs []string
}

func (f *fakeAgentPlanQuerier) GetPersonalPlan(plan string) (PersonalPlanResult, error) {
	f.gotPlan = plan
	return f.personal, f.personalErr
}

func (f *fakeAgentPlanQuerier) GetSeatAFPUsage(seatIDs []string) ([]SeatAFPUsage, error) {
	f.gotSeatIDs = seatIDs
	return f.usages, f.usagesErr
}

func TestAgentPlanType(t *testing.T) {
	t.Run("personal edition queries the personal plan and normalizes to lower case", func(t *testing.T) {
		q := &fakeAgentPlanQuerier{personal: PersonalPlanResult{PlanType: "Small"}}
		got, err := resolveAgentPlanType(q, CreateWorkspaceParams{IsAgentPlan: boolPtr(true)})
		require.NoError(t, err)
		assert.Equal(t, "small", got)
		assert.Equal(t, PlanTypeAgentPlan, q.gotPlan)
	})

	t.Run("personal edition propagates the lookup error", func(t *testing.T) {
		q := &fakeAgentPlanQuerier{personalErr: errors.New("boom")}
		_, err := resolveAgentPlanType(q, CreateWorkspaceParams{IsAgentPlan: boolPtr(true)})
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("enterprise edition matches the requested seat and trims the seat ID", func(t *testing.T) {
		q := &fakeAgentPlanQuerier{usages: []SeatAFPUsage{
			{SeatID: "seat-other", PlanType: "Large"},
			{SeatID: "seat-123", PlanType: "Medium"},
		}}
		got, err := resolveAgentPlanType(q, CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "  seat-123  "})
		require.NoError(t, err)
		assert.Equal(t, "medium", got)
		assert.Equal(t, []string{"seat-123"}, q.gotSeatIDs)
	})

	t.Run("enterprise edition errors when the requested seat is absent", func(t *testing.T) {
		q := &fakeAgentPlanQuerier{usages: []SeatAFPUsage{{SeatID: "seat-other", PlanType: "Small"}}}
		_, err := resolveAgentPlanType(q, CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "seat-123"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "seat-123")
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("enterprise edition errors on an empty response instead of panicking", func(t *testing.T) {
		q := &fakeAgentPlanQuerier{usages: nil}
		_, err := resolveAgentPlanType(q, CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "seat-123"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("enterprise edition propagates the lookup error", func(t *testing.T) {
		q := &fakeAgentPlanQuerier{usagesErr: errors.New("boom")}
		_, err := resolveAgentPlanType(q, CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "seat-123"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "boom")
	})
}

func TestIsPersonalAgentPlan(t *testing.T) {
	assert.True(t, IsPersonalAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(true)}))
	assert.True(t, IsPersonalAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "   "}))

	// Enterprise edition: a seat ID means it is not a personal plan.
	assert.False(t, IsPersonalAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "seat-123"}))
	// Agent Plan disabled or unspecified.
	assert.False(t, IsPersonalAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(false)}))
	assert.False(t, IsPersonalAgentPlan(CreateWorkspaceParams{}))
}

func TestIsAgentPlan(t *testing.T) {
	assert.True(t, IsAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(true)}))
	assert.True(t, IsAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(true), AgentPlanSeatID: "seat-123"}))
	assert.False(t, IsAgentPlan(CreateWorkspaceParams{IsAgentPlan: boolPtr(false)}))
	assert.False(t, IsAgentPlan(CreateWorkspaceParams{}))
}

func TestIsSmallAgentPlanType(t *testing.T) {
	// Only the small sizes (Small/Medium) qualify for the 60-minute default. The match is
	// case-insensitive: GetPersonalPlan returns "Small"/"Medium" while GetSeatAFPUsage
	// returns "small"/"medium".
	for _, planType := range []string{"Small", "Medium", "small", "medium", " Small ", "SMALL"} {
		assert.Truef(t, IsSmallAgentPlanType(planType), "expected %q to be a small plan", planType)
	}
	// Larger sizes and unknown values are left to the backend default.
	for _, planType := range []string{"Large", "large", "Max", "max", "", "Tiny", "Unknown"} {
		assert.Falsef(t, IsSmallAgentPlanType(planType), "expected %q not to be a small plan", planType)
	}
}
