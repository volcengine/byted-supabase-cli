// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/volcengine-go-sdk/volcengine/universal"
)

const (
	// arkServiceName and arkAPIVersion identify the Ark OpenAPI service that hosts the
	// Agent Plan quota query actions (GetPersonalPlan, GetSeatAFPUsage). These are NOT on
	// the aidap service that the workspace operations use.
	arkServiceName = "ark"
	arkAPIVersion  = "2024-01-01"
	// arkRegion pins the signing region for Ark plan/quota queries. The data is
	// account-global; cn-beijing is the documented region for these actions.
	arkRegion = "cn-beijing"

	// PlanTypeAgentPlan and PlanTypeCodingPlan are the valid GetPersonalPlan "Plan"
	// request values.
	PlanTypeAgentPlan  = "AgentPlan"
	PlanTypeCodingPlan = "CodingPlan"

	// SmallAgentPlanSuspendTimeoutSeconds is the auto-suspend idle timeout (60 min)
	// defaulted onto small (Small/Medium) Agent Plan workspaces created without an
	// explicit --suspend-timeout-seconds.
	SmallAgentPlanSuspendTimeoutSeconds = 3600
)

type getPersonalPlanReq struct {
	Plan string `json:"Plan"`
}

// PersonalPlanResult is the GetPersonalPlan Result payload describing the account's
// active personal plan.
type PersonalPlanResult struct {
	PlanType  string `json:"PlanType"`
	Status    string `json:"Status"`
	StartTime string `json:"StartTime"`
	EndTime   string `json:"EndTime"`
	AutoRenew bool   `json:"AutoRenew"`
}

// GetPersonalPlan returns the account's active personal plan for the given plan kind
// (AgentPlan or CodingPlan). It returns an error if the user has no active or existing
// personal plan (ResourceNotFound.Plan).
func (c *Client) GetPersonalPlan(plan string) (PersonalPlanResult, error) {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		plan = PlanTypeAgentPlan
	}
	var out PersonalPlanResult
	if err := c.callArkUniversal("GetPersonalPlan", &getPersonalPlanReq{Plan: plan}, &out); err != nil {
		return PersonalPlanResult{}, errors.Errorf("failed to call volcengine GetPersonalPlan: %w", err)
	}
	return out, nil
}

type getSeatAFPUsageReq struct {
	SeatIDs []string `json:"SeatIDs"`
}

// AFPUsageWindow describes the AFP quota usage for a single billing window.
type AFPUsageWindow struct {
	Quota         float64 `json:"Quota"`
	Used          float64 `json:"Used"`
	SubscribeTime int64   `json:"SubscribeTime"`
	ResetTime     int64   `json:"ResetTime"`
}

// SeatAFPUsage is the per-seat AFP usage (and plan size) for an enterprise Agent Plan.
type SeatAFPUsage struct {
	SeatID      string         `json:"SeatID"`
	PlanType    string         `json:"PlanType"`
	AFPFiveHour AFPUsageWindow `json:"AFPFiveHour"`
	AFPDaily    AFPUsageWindow `json:"AFPDaily"`
	AFPWeekly   AFPUsageWindow `json:"AFPWeekly"`
	AFPMonthly  AFPUsageWindow `json:"AFPMonthly"`
}

type getSeatAFPUsageResult struct {
	SeatAFPUsages []SeatAFPUsage `json:"SeatAFPUsages"`
}

// GetSeatAFPUsage returns the AFP quota usage (and plan size) for the given enterprise
// Agent Plan seat IDs. A single request accepts at most 1000 seat IDs.
func (c *Client) GetSeatAFPUsage(seatIDs []string) ([]SeatAFPUsage, error) {
	var out getSeatAFPUsageResult
	if err := c.callArkUniversal("GetSeatAFPUsage", &getSeatAFPUsageReq{SeatIDs: seatIDs}, &out); err != nil {
		return nil, errors.Errorf("failed to call volcengine GetSeatAFPUsage: %w", err)
	}
	return out.SeatAFPUsages, nil
}

// callArkUniversal invokes an Ark OpenAPI action through the dedicated ark universal
// client. Transitional path: these actions are not in the released strong-typed Go SDK.
func (c *Client) callArkUniversal(action string, input, output interface{}) error {
	return c.arkUni.DoCallWithType(universal.RequestUniversal{
		ServiceName: arkServiceName,
		Action:      action,
		Version:     arkAPIVersion,
		HttpMethod:  universal.POST,
		ContentType: universal.ApplicationJSON,
	}, input, output)
}

// agentPlanQuerier is the subset of Client used to resolve an Agent Plan's size. It lets
// AgentPlanType be unit-tested with a fake, without a live Ark endpoint.
type agentPlanQuerier interface {
	GetPersonalPlan(plan string) (PersonalPlanResult, error)
	GetSeatAFPUsage(seatIDs []string) ([]SeatAFPUsage, error)
}

// AgentPlanType resolves the plan size for a resolved Agent Plan workspace: the personal
// edition queries the account's personal plan, the enterprise edition queries the AFP
// usage for the workspace's seat ID. The result is normalized to lower case so callers
// are case-insensitive — GetPersonalPlan returns capitalized sizes (e.g. "Large") while
// GetSeatAFPUsage returns lower case (e.g. "small").
func (c *Client) AgentPlanType(params CreateWorkspaceParams) (string, error) {
	return resolveAgentPlanType(c, params)
}

func resolveAgentPlanType(q agentPlanQuerier, params CreateWorkspaceParams) (string, error) {
	raw, err := agentPlanTypeRaw(q, params)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(raw)), nil
}

func agentPlanTypeRaw(q agentPlanQuerier, params CreateWorkspaceParams) (string, error) {
	if IsPersonalAgentPlan(params) {
		plan, err := q.GetPersonalPlan(PlanTypeAgentPlan)
		if err != nil {
			return "", err
		}
		return plan.PlanType, nil
	}
	seatID := strings.TrimSpace(params.AgentPlanSeatID)
	usages, err := q.GetSeatAFPUsage([]string{seatID})
	if err != nil {
		return "", err
	}
	for _, usage := range usages {
		if usage.SeatID == seatID {
			return usage.PlanType, nil
		}
	}
	return "", errors.Errorf("seat %q not found in GetSeatAFPUsage response", seatID)
}

// IsAgentPlan reports whether the resolved create params describe an Agent Plan
// workspace (personal or enterprise edition).
func IsAgentPlan(params CreateWorkspaceParams) bool {
	return params.IsAgentPlan != nil && *params.IsAgentPlan
}

// IsPersonalAgentPlan reports whether the resolved create params describe a
// personal-edition Agent Plan workspace (Agent Plan enabled with no enterprise seat ID).
func IsPersonalAgentPlan(params CreateWorkspaceParams) bool {
	return IsAgentPlan(params) && strings.TrimSpace(params.AgentPlanSeatID) == ""
}

// IsSmallAgentPlanType reports whether planType is a "small" Agent Plan size (Small or
// Medium). Only these sizes receive the 60-minute auto-suspend default; the larger sizes
// (Large, Max) are left to the backend default. Applies to both personal and enterprise
// editions.
//
// The match is case-insensitive: GetPersonalPlan returns capitalized sizes (e.g. "Large")
// while GetSeatAFPUsage returns lowercase (e.g. "small"), despite the docs listing only
// the capitalized forms.
func IsSmallAgentPlanType(planType string) bool {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "small", "medium":
		return true
	default:
		return false
	}
}
