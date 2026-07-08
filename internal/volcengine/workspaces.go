// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/volcengine/volcengine-go-sdk/service/aidap"
)

const (
	EngineTypeSupabase = "Supabase"
	maxWorkspaceLimit  = 100
	// DefaultListLimit is the default page size used by list endpoints when the caller
	// does not specify a count. The backend pagination API defaults to 10 when Limit is
	// omitted; this constant makes that explicit and is shared by MCP list tools (as the
	// default for the count parameter) and fork-specific DescribeDatabases/DescribeDBAccounts.
	DefaultListLimit = 10
)

type ListWorkspacesParams struct {
	Search      string
	ProjectName string
	Limit       int
	Offset      int
}

type WorkspaceOverviewParams struct {
	ProjectName string
}

type CreateWorkspaceParams struct {
	WorkspaceName         string
	ProjectName           string
	IsAgentPlan           *bool
	AgentPlanSeatID       string
	SuspendTimeoutSeconds *int
}

type WorkspaceTag struct {
	Key    string
	Value  string
	System bool
}

type ModifyComputeSettingsParams struct {
	AutoScalingLimitMinCU float64
	AutoScalingLimitMaxCU float64
	SuspendTimeoutSeconds *int
	ServiceType           string
}

type ModifyWorkspaceSettingsParams struct {
	HistoryRetentionHours int
}

type ListWorkspacesResult struct {
	Total      int
	Workspaces []Workspace `json:"Workspaces"`
}

type CreateWorkspaceResult struct {
	WorkspaceID string    `json:"WorkspaceId"`
	Workspace   Workspace `json:"Workspace"`
}

type DeleteWorkspaceResult struct {
	WorkspaceID string `json:"WorkspaceId"`
}

type WorkspaceOverviewResult struct {
	Overviews []WorkspaceOverview `json:"WorkspaceOverviews"`
}

type WorkspaceOverview struct {
	EngineType     string `json:"EngineType"`
	WorkspaceTotal int    `json:"WorkspaceTotal"`
	RunningTotal   int    `json:"RunningTotal"`
	CreatingTotal  int    `json:"CreatingTotal"`
	UpdatingTotal  int    `json:"UpdatingTotal"`
	StoppedTotal   int    `json:"StoppedTotal"`
	SuspendedTotal int    `json:"SuspendedTotal"`
	ClosedTotal    int    `json:"ClosedTotal"`
	ErrorTotal     int    `json:"ErrorTotal"`
}

type Workspace struct {
	WorkspaceID              string                   `json:"WorkspaceId"`
	WorkspaceName            string                   `json:"WorkspaceName"`
	RegionID                 string                   `json:"RegionId"`
	ProjectName              string                   `json:"ProjectName"`
	AccountID                string                   `json:"AccountId"`
	EngineType               string                   `json:"EngineType"`
	EngineVersion            string                   `json:"EngineVersion"`
	WorkspaceStatus          string                   `json:"WorkspaceStatus"`
	CreateTime               string                   `json:"CreateTime"`
	UpdateTime               string                   `json:"UpdateTime"`
	CreationSource           string                   `json:"CreationSource"`
	DeletionProtectionStatus string                   `json:"DeletionProtectionStatus"`
	InternetProtocol         string                   `json:"InternetProtocol"`
	DNSVisibility            bool                     `json:"DNSVisibility"`
	SharedPrivateNetwork     bool                     `json:"SharedPrivateNetwork"`
	StorageSize              int                      `json:"StorageSize"`
	StorageType              string                   `json:"StorageType"`
	IsAgentPlan              bool                     `json:"IsAgentPlan"`
	IsAgentPlanInstance      bool                     `json:"IsAgentPlanInstance"`
	AgentPlanSeatID          string                   `json:"AgentPlanSeatId"`
	VpcID                    string                   `json:"VpcId"`
	SubnetID                 string                   `json:"SubnetId"`
	WorkspaceSetting         WorkspaceSetting         `json:"WorkspaceSetting"`
	ComputeSettings          WorkspaceComputeSettings `json:"ComputeSettings"`
	WorkspaceUsage           WorkspaceUsage           `json:"WorkspaceUsage"`
	WorkspaceTags            []WorkspaceTag           `json:"WorkspaceTags"`
}

type WorkspaceSetting struct {
	DeletionProtection    string `json:"DeletionProtection"`
	HistoryRetentionHours int    `json:"HistoryRetentionHours"`
	PublicConnection      string `json:"PublicConnection"`
}

type WorkspaceComputeSettings struct {
	AutoScalingLimitMinCU float64 `json:"AutoScalingLimitMinCU"`
	AutoScalingLimitMaxCU float64 `json:"AutoScalingLimitMaxCU"`
	EnableAnalytics       string  `json:"EnableAnalytics"`
	SuspendTimeoutSeconds int     `json:"SuspendTimeoutSeconds"`
}

type WorkspaceUsage struct {
	DataSizeTotalBytes   int64  `json:"DataSizeTotalBytes"`
	DataSizeUsedBytes    int64  `json:"DataSizeUsedBytes"`
	StorageSizeUsedBytes int64  `json:"StorageSizeUsedBytes"`
	BranchCreatedNum     int64  `json:"BranchCreatedNum"`
	FunctionCallNum      int64  `json:"FunctionCallNum"`
	StatTime             string `json:"StatTime"`
}

func (c *Client) ListWorkspaces(ctx context.Context, params ListWorkspacesParams) (ListWorkspacesResult, error) {
	limit := params.Limit
	if limit == 0 {
		limit = maxWorkspaceLimit
	}
	req := (&aidap.DescribeWorkspacesInput{}).
		SetSortBy("update_time").
		SetSortOrder(aidap.EnumOfSortOrderForDescribeWorkspacesInputDesc).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset)).
		SetFilters([]*aidap.FilterForDescribeWorkspacesInput{
			(&aidap.FilterForDescribeWorkspacesInput{}).
				SetName("EngineType").
				SetValue(aidap.EnumOfEngineTypeForDescribeWorkspacesOutputSupabase),
		})
	if params.Search != "" {
		req.SetSearch(params.Search)
	}
	if params.ProjectName != "" {
		req.SetProjectName(params.ProjectName)
	}
	resp, err := c.aidap.DescribeWorkspacesWithContext(ctx, req)
	if err != nil {
		return ListWorkspacesResult{}, errors.Errorf("failed to call volcengine DescribeWorkspaces: %w", err)
	}
	return mapListWorkspacesResult(resp), nil
}

func (c *Client) ListAllSupabaseWorkspaces(ctx context.Context, params ListWorkspacesParams) (ListWorkspacesResult, error) {
	limit := params.Limit
	if limit == 0 || limit > maxWorkspaceLimit {
		limit = maxWorkspaceLimit
	}
	params.Limit = limit

	var result ListWorkspacesResult
	for {
		page, err := c.ListWorkspaces(ctx, params)
		if err != nil {
			return ListWorkspacesResult{}, err
		}
		result.Total = page.Total
		result.Workspaces = append(result.Workspaces, page.Workspaces...)
		if len(result.Workspaces) >= page.Total || len(page.Workspaces) == 0 {
			return result, nil
		}
		params.Offset += limit
	}
}

func (c *Client) CreateSupabaseWorkspace(ctx context.Context, params CreateWorkspaceParams) (CreateWorkspaceResult, error) {
	params, err := ResolveCreateWorkspaceAgentPlan(params)
	if err != nil {
		return CreateWorkspaceResult{}, err
	}
	req := (&aidap.CreateWorkspaceInput{}).
		SetEngineType(aidap.EnumOfEngineTypeForCreateWorkspaceInputSupabase).
		SetEngineVersion(aidap.EnumOfEngineVersionForCreateWorkspaceInputSupabase124)
	if params.WorkspaceName != "" {
		req.SetWorkspaceName(params.WorkspaceName)
	}
	if params.ProjectName != "" {
		req.SetProjectName(params.ProjectName)
	}
	if settings := buildAgentPlanSettings(params); settings != nil {
		req.SetAgentPlanSettings(settings)
	}
	if params.SuspendTimeoutSeconds != nil {
		req.SetComputeSettings((&aidap.ComputeSettingsForCreateWorkspaceInput{}).SetSuspendTimeoutSeconds(int32(*params.SuspendTimeoutSeconds)))
	}
	resp, err := c.aidap.CreateWorkspaceWithContext(ctx, req)
	if err != nil {
		return CreateWorkspaceResult{}, errors.Errorf("failed to call volcengine CreateWorkspace: %w", err)
	}
	return mapCreateWorkspaceResult(resp), nil
}

func buildAgentPlanSettings(params CreateWorkspaceParams) *aidap.AgentPlanSettingsForCreateWorkspaceInput {
	if params.IsAgentPlan == nil && params.AgentPlanSeatID == "" {
		return nil
	}
	settings := &aidap.AgentPlanSettingsForCreateWorkspaceInput{}
	if params.AgentPlanSeatID != "" {
		settings.SetIsAgentPlan(true)
	} else if params.IsAgentPlan != nil {
		settings.SetIsAgentPlan(*params.IsAgentPlan)
	}
	if params.AgentPlanSeatID != "" {
		settings.SetAgentPlanSeatId(params.AgentPlanSeatID)
	}
	return settings
}

func (c *Client) DeleteWorkspace(ctx context.Context, workspaceID string) (DeleteWorkspaceResult, error) {
	resp, err := c.aidap.DeleteWorkspaceWithContext(ctx, (&aidap.DeleteWorkspaceInput{}).SetWorkspaceId(workspaceID))
	if err != nil {
		return DeleteWorkspaceResult{}, errors.Errorf("failed to call volcengine DeleteWorkspace: %w", err)
	}
	if resp == nil {
		return DeleteWorkspaceResult{}, nil
	}
	return DeleteWorkspaceResult{WorkspaceID: stringValue(resp.WorkspaceId)}, nil
}

func (c *Client) StartWorkspace(ctx context.Context, workspaceID string) error {
	if _, err := c.aidap.StartWorkspaceWithContext(ctx, (&aidap.StartWorkspaceInput{}).SetWorkspaceId(workspaceID)); err != nil {
		return errors.Errorf("failed to call volcengine StartWorkspace: %w", err)
	}
	return nil
}

func (c *Client) StopWorkspace(ctx context.Context, workspaceID string) error {
	if _, err := c.aidap.StopWorkspaceWithContext(ctx, (&aidap.StopWorkspaceInput{}).SetWorkspaceId(workspaceID)); err != nil {
		return errors.Errorf("failed to call volcengine StopWorkspace: %w", err)
	}
	return nil
}

func (c *Client) ModifyWorkspaceName(ctx context.Context, workspaceID, workspaceName string) error {
	if _, err := c.aidap.ModifyWorkspaceNameWithContext(ctx, (&aidap.ModifyWorkspaceNameInput{}).
		SetWorkspaceId(workspaceID).
		SetWorkspaceName(workspaceName)); err != nil {
		return errors.Errorf("failed to call volcengine ModifyWorkspaceName: %w", err)
	}
	return nil
}

func (c *Client) ModifyWorkspaceDeletionProtectionPolicy(ctx context.Context, workspaceID string, enabled bool) error {
	deletionProtection := aidap.EnumOfDeletionProtectionForModifyWorkspaceDeletionProtectionPolicyInputDisabled
	if enabled {
		deletionProtection = aidap.EnumOfDeletionProtectionForModifyWorkspaceDeletionProtectionPolicyInputEnabled
	}
	if _, err := c.aidap.ModifyWorkspaceDeletionProtectionPolicyWithContext(ctx, (&aidap.ModifyWorkspaceDeletionProtectionPolicyInput{}).
		SetWorkspaceId(workspaceID).
		SetDeletionProtection(deletionProtection)); err != nil {
		return errors.Errorf("failed to call volcengine ModifyWorkspaceDeletionProtectionPolicy: %w", err)
	}
	return nil
}

func (c *Client) AddTagsToWorkspaces(ctx context.Context, workspaceIDs []string, tags []WorkspaceTag) error {
	reqTags := make([]*aidap.TagForAddTagsToWorkspacesInput, 0, len(tags))
	for _, tag := range tags {
		reqTags = append(reqTags, (&aidap.TagForAddTagsToWorkspacesInput{}).
			SetKey(tag.Key).
			SetValue(tag.Value))
	}
	if _, err := c.aidap.AddTagsToWorkspacesWithContext(ctx, (&aidap.AddTagsToWorkspacesInput{}).
		SetWorkspaceIds(stringPointers(workspaceIDs)).
		SetTags(reqTags)); err != nil {
		return errors.Errorf("failed to call volcengine AddTagsToWorkspaces: %w", err)
	}
	return nil
}

func (c *Client) RemoveTagsFromWorkspaces(ctx context.Context, workspaceIDs []string, tagKeys []string) error {
	if _, err := c.aidap.RemoveTagsFromWorkspacesWithContext(ctx, (&aidap.RemoveTagsFromWorkspacesInput{}).
		SetWorkspaceIds(stringPointers(workspaceIDs)).
		SetTagKeys(stringPointers(tagKeys))); err != nil {
		return errors.Errorf("failed to call volcengine RemoveTagsFromWorkspaces: %w", err)
	}
	return nil
}

func (c *Client) ModifyComputeSettings(ctx context.Context, workspaceID string, params ModifyComputeSettingsParams) (Workspace, error) {
	req := (&aidap.ModifyComputeSettingsInput{}).
		SetWorkspaceId(workspaceID).
		SetAutoScalingLimitMinCU(params.AutoScalingLimitMinCU).
		SetAutoScalingLimitMaxCU(params.AutoScalingLimitMaxCU)
	if params.ServiceType != "" {
		req.SetServiceType(params.ServiceType)
	}
	if params.SuspendTimeoutSeconds != nil {
		req.SetSuspendTimeoutSeconds(int32(*params.SuspendTimeoutSeconds))
	}
	resp, err := c.aidap.ModifyComputeSettingsWithContext(ctx, req)
	if err != nil {
		return Workspace{}, errors.Errorf("failed to call volcengine ModifyComputeSettings: %w", err)
	}
	if resp == nil {
		return Workspace{}, nil
	}
	return mapWorkspaceFromModifyComputeSettings(resp.Workspace), nil
}

func (c *Client) ModifyWorkspaceSettings(ctx context.Context, workspaceID string, params ModifyWorkspaceSettingsParams) (Workspace, error) {
	req := (&aidap.ModifyWorkspaceSettingsInput{}).
		SetWorkspaceId(workspaceID).
		SetWorkspaceSettings((&aidap.WorkspaceSettingsForModifyWorkspaceSettingsInput{}).
			SetHistoryRetentionHours(int32(params.HistoryRetentionHours)))
	resp, err := c.aidap.ModifyWorkspaceSettingsWithContext(ctx, req)
	if err != nil {
		return Workspace{}, errors.Errorf("failed to call volcengine ModifyWorkspaceSettings: %w", err)
	}
	if resp == nil {
		return Workspace{}, nil
	}
	return mapWorkspaceFromModifyWorkspaceSettings(resp.Workspace), nil
}

func (c *Client) DescribeSupabaseWorkspaceOverview(ctx context.Context, params WorkspaceOverviewParams) (WorkspaceOverviewResult, error) {
	req := (&aidap.DescribeWorkspaceOverviewInput{}).
		SetEngineType(aidap.EnumOfEngineTypeForDescribeWorkspaceOverviewInputSupabase)
	if params.ProjectName != "" {
		req.SetProjectName(params.ProjectName)
	}
	resp, err := c.aidap.DescribeWorkspaceOverviewWithContext(ctx, req)
	if err != nil {
		return WorkspaceOverviewResult{}, errors.Errorf("failed to call volcengine DescribeWorkspaceOverview: %w", err)
	}
	return mapWorkspaceOverviewResult(resp), nil
}

func mapCreateWorkspaceResult(resp *aidap.CreateWorkspaceOutput) CreateWorkspaceResult {
	if resp == nil {
		return CreateWorkspaceResult{}
	}
	return CreateWorkspaceResult{
		WorkspaceID: stringValue(resp.WorkspaceId),
		Workspace:   mapWorkspaceFromCreate(resp.Workspace),
	}
}

func mapListWorkspacesResult(resp *aidap.DescribeWorkspacesOutput) ListWorkspacesResult {
	if resp == nil {
		return ListWorkspacesResult{}
	}
	result := ListWorkspacesResult{
		Total: intValue(resp.Total),
	}
	for _, workspace := range resp.Workspaces {
		result.Workspaces = append(result.Workspaces, mapWorkspaceFromList(workspace))
	}
	return result
}

func mapWorkspaceFromCreate(workspace *aidap.WorkspaceForCreateWorkspaceOutput) Workspace {
	if workspace == nil {
		return Workspace{}
	}
	result := Workspace{
		WorkspaceID:              stringValue(workspace.WorkspaceId),
		WorkspaceName:            stringValue(workspace.WorkspaceName),
		RegionID:                 stringValue(workspace.RegionId),
		ProjectName:              stringValue(workspace.ProjectName),
		AccountID:                stringValue(workspace.AccountId),
		EngineType:               stringValue(workspace.EngineType),
		EngineVersion:            stringValue(workspace.EngineVersion),
		WorkspaceStatus:          stringValue(workspace.WorkspaceStatus),
		CreateTime:               stringValue(workspace.CreateTime),
		UpdateTime:               stringValue(workspace.UpdateTime),
		CreationSource:           stringValue(workspace.CreationSource),
		DeletionProtectionStatus: stringValue(workspace.DeletionProtectionStatus),
		InternetProtocol:         stringValue(workspace.InternetProtocol),
		DNSVisibility:            boolValue(workspace.DNSVisibility),
		SharedPrivateNetwork:     boolValue(workspace.SharedPrivateNetwork),
		VpcID:                    stringValue(workspace.VpcId),
		SubnetID:                 stringValue(workspace.SubnetId),
	}
	if workspace.WorkspaceSetting != nil {
		result.WorkspaceSetting = WorkspaceSetting{
			DeletionProtection:    stringValue(workspace.WorkspaceSetting.DeletionProtection),
			HistoryRetentionHours: intValue(workspace.WorkspaceSetting.HistoryRetentionHours),
			PublicConnection:      stringValue(workspace.WorkspaceSetting.PublicConnection),
		}
	}
	if workspace.ComputeSettings != nil {
		result.ComputeSettings = WorkspaceComputeSettings{
			AutoScalingLimitMinCU: floatValue(workspace.ComputeSettings.AutoScalingLimitMinCU),
			AutoScalingLimitMaxCU: floatValue(workspace.ComputeSettings.AutoScalingLimitMaxCU),
			EnableAnalytics:       stringValue(workspace.ComputeSettings.EnableAnalytic),
			SuspendTimeoutSeconds: intValue(workspace.ComputeSettings.SuspendTimeoutSeconds),
		}
	}
	if workspace.WorkspaceUsage != nil {
		result.WorkspaceUsage = WorkspaceUsage{
			DataSizeTotalBytes:   int64Value(workspace.WorkspaceUsage.DataSizeTotalBytes),
			DataSizeUsedBytes:    int64Value(workspace.WorkspaceUsage.DataSizeUsedBytes),
			StorageSizeUsedBytes: int64Value(workspace.WorkspaceUsage.StorageSizeUsedBytes),
			BranchCreatedNum:     int64Value(workspace.WorkspaceUsage.BranchCreatedNum),
			FunctionCallNum:      int64Value(workspace.WorkspaceUsage.FunctionCallNum),
			StatTime:             stringValue(workspace.WorkspaceUsage.StatTime),
		}
	}
	result.IsAgentPlan = boolValue(workspace.IsAgentPlan)
	result.IsAgentPlanInstance = boolValue(workspace.IsAgentPlanInstance)
	result.AgentPlanSeatID = stringValue(workspace.AgentPlanSeatId)
	return result
}

func mapWorkspaceFromList(workspace *aidap.WorkspaceForDescribeWorkspacesOutput) Workspace {
	if workspace == nil {
		return Workspace{}
	}
	result := Workspace{
		WorkspaceID:     stringValue(workspace.WorkspaceId),
		WorkspaceName:   stringValue(workspace.WorkspaceName),
		RegionID:        stringValue(workspace.RegionId),
		ProjectName:     stringValue(workspace.ProjectName),
		AccountID:       stringValue(workspace.AccountId),
		EngineType:      stringValue(workspace.EngineType),
		EngineVersion:   stringValue(workspace.EngineVersion),
		WorkspaceStatus: stringValue(workspace.WorkspaceStatus),
		CreateTime:      stringValue(workspace.CreateTime),
		UpdateTime:      stringValue(workspace.UpdateTime),
	}
	result.IsAgentPlan = boolValue(workspace.IsAgentPlan)
	result.IsAgentPlanInstance = boolValue(workspace.IsAgentPlanInstance)
	result.AgentPlanSeatID = stringValue(workspace.AgentPlanSeatId)
	return result
}

func mapWorkspaceOverviewResult(resp *aidap.DescribeWorkspaceOverviewOutput) WorkspaceOverviewResult {
	if resp == nil {
		return WorkspaceOverviewResult{}
	}
	result := WorkspaceOverviewResult{}
	for _, overview := range resp.WorkspaceOverviews {
		result.Overviews = append(result.Overviews, mapWorkspaceOverview(overview))
	}
	return result
}

func mapWorkspaceOverview(overview *aidap.WorkspaceOverviewForDescribeWorkspaceOverviewOutput) WorkspaceOverview {
	if overview == nil {
		return WorkspaceOverview{}
	}
	return WorkspaceOverview{
		EngineType:     stringValue(overview.EngineType),
		WorkspaceTotal: intValue(overview.WorkspaceTotal),
		RunningTotal:   intValue(overview.RunningTotal),
		CreatingTotal:  intValue(overview.CreatingTotal),
		UpdatingTotal:  intValue(overview.UpdatingTotal),
		StoppedTotal:   intValue(overview.StoppedTotal),
		SuspendedTotal: intValue(overview.SuspendedTotal),
		ClosedTotal:    intValue(overview.ClosedTotal),
		ErrorTotal:     intValue(overview.ErrorTotal),
	}
}

func mapWorkspaceFromModifyComputeSettings(workspace *aidap.WorkspaceForModifyComputeSettingsOutput) Workspace {
	if workspace == nil {
		return Workspace{}
	}
	result := Workspace{
		WorkspaceID:              stringValue(workspace.WorkspaceId),
		WorkspaceName:            stringValue(workspace.WorkspaceName),
		RegionID:                 stringValue(workspace.RegionId),
		ProjectName:              stringValue(workspace.ProjectName),
		AccountID:                stringValue(workspace.AccountId),
		EngineType:               stringValue(workspace.EngineType),
		EngineVersion:            stringValue(workspace.EngineVersion),
		WorkspaceStatus:          stringValue(workspace.WorkspaceStatus),
		CreateTime:               stringValue(workspace.CreateTime),
		UpdateTime:               stringValue(workspace.UpdateTime),
		CreationSource:           stringValue(workspace.CreationSource),
		DeletionProtectionStatus: stringValue(workspace.DeletionProtectionStatus),
		InternetProtocol:         stringValue(workspace.InternetProtocol),
		DNSVisibility:            boolValue(workspace.DNSVisibility),
		SharedPrivateNetwork:     boolValue(workspace.SharedPrivateNetwork),
		VpcID:                    stringValue(workspace.VpcId),
		SubnetID:                 stringValue(workspace.SubnetId),
	}
	if workspace.WorkspaceSetting != nil {
		result.WorkspaceSetting = WorkspaceSetting{
			DeletionProtection:    stringValue(workspace.WorkspaceSetting.DeletionProtection),
			HistoryRetentionHours: intValue(workspace.WorkspaceSetting.HistoryRetentionHours),
			PublicConnection:      stringValue(workspace.WorkspaceSetting.PublicConnection),
		}
	}
	if workspace.ComputeSettings != nil {
		result.ComputeSettings = WorkspaceComputeSettings{
			AutoScalingLimitMinCU: floatValue(workspace.ComputeSettings.AutoScalingLimitMinCU),
			AutoScalingLimitMaxCU: floatValue(workspace.ComputeSettings.AutoScalingLimitMaxCU),
			SuspendTimeoutSeconds: intValue(workspace.ComputeSettings.SuspendTimeoutSeconds),
		}
	}
	return result
}

func mapWorkspaceFromModifyWorkspaceSettings(workspace *aidap.WorkspaceForModifyWorkspaceSettingsOutput) Workspace {
	if workspace == nil {
		return Workspace{}
	}
	result := Workspace{
		WorkspaceID:              stringValue(workspace.WorkspaceId),
		WorkspaceName:            stringValue(workspace.WorkspaceName),
		RegionID:                 stringValue(workspace.RegionId),
		ProjectName:              stringValue(workspace.ProjectName),
		AccountID:                stringValue(workspace.AccountId),
		EngineType:               stringValue(workspace.EngineType),
		EngineVersion:            stringValue(workspace.EngineVersion),
		WorkspaceStatus:          stringValue(workspace.WorkspaceStatus),
		CreateTime:               stringValue(workspace.CreateTime),
		UpdateTime:               stringValue(workspace.UpdateTime),
		CreationSource:           stringValue(workspace.CreationSource),
		DeletionProtectionStatus: stringValue(workspace.DeletionProtectionStatus),
		InternetProtocol:         stringValue(workspace.InternetProtocol),
		DNSVisibility:            boolValue(workspace.DNSVisibility),
		SharedPrivateNetwork:     boolValue(workspace.SharedPrivateNetwork),
		VpcID:                    stringValue(workspace.VpcId),
		SubnetID:                 stringValue(workspace.SubnetId),
	}
	if workspace.WorkspaceSetting != nil {
		result.WorkspaceSetting = WorkspaceSetting{
			DeletionProtection:    stringValue(workspace.WorkspaceSetting.DeletionProtection),
			HistoryRetentionHours: intValue(workspace.WorkspaceSetting.HistoryRetentionHours),
			PublicConnection:      stringValue(workspace.WorkspaceSetting.PublicConnection),
		}
	}
	if workspace.ComputeSettings != nil {
		result.ComputeSettings = WorkspaceComputeSettings{
			AutoScalingLimitMinCU: floatValue(workspace.ComputeSettings.AutoScalingLimitMinCU),
			AutoScalingLimitMaxCU: floatValue(workspace.ComputeSettings.AutoScalingLimitMaxCU),
			SuspendTimeoutSeconds: intValue(workspace.ComputeSettings.SuspendTimeoutSeconds),
		}
	}
	return result
}

func stringPointers(values []string) []*string {
	result := make([]*string, 0, len(values))
	for _, value := range values {
		value := value
		result = append(result, &value)
	}
	return result
}
