// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/volcengine-go-sdk/service/aidap"
	"github.com/volcengine/volcengine-go-sdk/service/vpc"
)

const (
	ServiceTypeDatabase = "Database"
	ServiceTypeSupabase = "Supabase"
	ComputeRolePrimary  = "Primary"
	ACLTypeAllow        = "Allow"
	ACLModifyModeCover  = "Cover"
	ACLModifyModeAppend = "Append"
	ACLModifyModeDelete = "Delete"
)

type DescribeWorkspaceDetailResult struct {
	Workspace Workspace
}

type DescribeAPIKeysParams struct {
	WorkspaceID string
	BranchID    string
	Limit       int
	Offset      int
}

type DescribeAPIKeysResult struct {
	Total   int
	APIKeys []APIKey
}

type DescribeOperationsParams struct {
	WorkspaceID     string
	BranchID        string
	ComputeID       string
	ActionName      string
	Status          string
	CreateTimeStart string
	CreateTimeEnd   string
	Limit           int
	Offset          int
}

type DescribeOperationsResult struct {
	Total      int         `json:"Total"`
	Operations []Operation `json:"Operations"`
}

type Operation struct {
	OperationID  string `json:"OperationId"`
	WorkspaceID  string `json:"WorkspaceId"`
	BranchID     string `json:"BranchId"`
	ComputeID    string `json:"ComputeId"`
	ActionName   string `json:"ActionName"`
	ActionStatus string `json:"ActionStatus"`
	CreateTime   string `json:"CreateTime"`
	FinishTime   string `json:"FinishTime,omitempty"`
	DurationTime string `json:"DurationTime,omitempty"`
}

type DescribeDefaultBranchResult struct {
	Branch Branch
}

type DescribeWorkspaceEndpointsParams struct {
	WorkspaceID string
	BranchID    string
}

type DescribeWorkspaceEndpointsResult struct {
	WorkspaceID string     `json:"WorkspaceId"`
	BranchID    string     `json:"BranchId"`
	Endpoints   []Endpoint `json:"Endpoints"`
}

type Endpoint struct {
	EndpointID   string            `json:"EndpointId"`
	EndpointName string            `json:"EndpointName"`
	EndpointType string            `json:"EndpointType"`
	Addresses    []EndpointAddress `json:"Addresses"`
}

type EndpointAddress struct {
	AddressID     string `json:"AddressId"`
	AddressType   string `json:"AddressType"`
	AddressDomain string `json:"AddressDomain"`
	AddressPort   int    `json:"AddressPort"`
	IPAddress     string `json:"IPAddress"`
	IPv6Address   string `json:"IPv6Address"`
}

type DescribeEIPAddressesParams struct {
	AvailableOnly bool
	Limit         int
}

type CreateEndpointPublicAddressParams struct {
	WorkspaceID string
	BranchID    string
	EndpointID  string
	EIPID       string
}

type DeleteEndpointPublicAddressParams struct {
	WorkspaceID string
	BranchID    string
	EndpointID  string
}

type EIPAddress struct {
	AllocationID string `json:"AllocationId"`
	Address      string `json:"EipAddress"`
	Name         string `json:"Name"`
	Status       string `json:"Status"`
	InstanceID   string `json:"InstanceId"`
	InstanceType string `json:"InstanceType"`
	Bandwidth    int64  `json:"Bandwidth"`
	ISP          string `json:"ISP"`
}

type VPCNetwork struct {
	VPCID         string   `json:"VpcId"`
	VPCName       string   `json:"VpcName"`
	CIDRBlock     string   `json:"CidrBlock"`
	IPv6CIDRBlock string   `json:"Ipv6CidrBlock"`
	Status        string   `json:"Status"`
	ProjectName   string   `json:"ProjectName"`
	Default       bool     `json:"IsDefault"`
	SubnetIDs     []string `json:"SubnetIds"`
}

type Subnet struct {
	SubnetID                string `json:"SubnetId"`
	SubnetName              string `json:"SubnetName"`
	VPCID                   string `json:"VpcId"`
	CIDRBlock               string `json:"CidrBlock"`
	IPv6CIDRBlock           string `json:"Ipv6CidrBlock"`
	Status                  string `json:"Status"`
	ZoneID                  string `json:"ZoneId"`
	AvailableIPAddressCount int64  `json:"AvailableIpAddressCount"`
	Default                 bool   `json:"IsDefault"`
}

type ModifyVpcSettingsParams struct {
	WorkspaceID      string
	BranchID         string
	VPCID            string
	SubnetID         string
	InternetProtocol string
}

type DescribeBranchesParams struct {
	WorkspaceID string
	Search      string
	Limit       int
	Offset      int
}

type DescribeChildBranchesParams struct {
	WorkspaceID string
	ParentID    string
	Limit       int
	Offset      int
}

type DescribeBranchesResult struct {
	Total         int
	WorkspaceName string
	Branches      []Branch
}

type DescribeBranchDetailResult struct {
	WorkspaceName string       `json:"WorkspaceName"`
	Branch        BranchDetail `json:"Branch"`
}

type CreateBranchParams struct {
	WorkspaceID string
	Name        string
	ParentID    string
	ParentTime  string
}

type CreateBranchResult struct {
	WorkspaceID string       `json:"WorkspaceId"`
	BranchID    string       `json:"BranchId"`
	Branch      BranchDetail `json:"Branch"`
}

// NormalizedBranch returns the result's branch detail with its branch/workspace
// ids backfilled from the top-level fields when the API leaves the nested Branch
// sparse. Shared by the CLI create command and the MCP create_branch tool.
func (r CreateBranchResult) NormalizedBranch() BranchDetail {
	branch := r.Branch
	if branch.BranchID == "" {
		branch.BranchID = r.BranchID
	}
	if branch.WorkspaceID == "" {
		branch.WorkspaceID = r.WorkspaceID
	}
	return branch
}

type UpdateBranchParams struct {
	WorkspaceID string
	BranchID    string
	Name        *string
}

type UpdateBranchResult struct {
	Branch BranchDetail `json:"Branch"`
}

type DeleteBranchResult struct {
	WorkspaceID string `json:"WorkspaceId"`
	BranchID    string `json:"BranchId"`
}

type RestartBranchParams struct {
	WorkspaceID string
	BranchID    string
	ComputeIDs  []string
}

type RestartBranchResult struct {
	WorkspaceID string   `json:"WorkspaceId"`
	BranchID    string   `json:"BranchId"`
	ComputeIDs  []string `json:"ComputeIds,omitempty"`
}

type SetAsDefaultBranchResult struct {
	Branch BranchDetail `json:"Branch"`
}

type RestoreWindow struct {
	WorkspaceID        string `json:"WorkspaceId,omitempty"`
	BranchID           string `json:"BranchId"`
	WindowSizeSeconds  int64  `json:"WindowSizeSeconds"`
	BranchCreationTime string `json:"BranchCreateTime"`
	StartTime          string `json:"StartTime"`
	EndTime            string `json:"EndTime"`
}

type DescribeRestorableBranchesParams struct {
	WorkspaceID string
	Time        string
	Search      string
	Limit       int
	Offset      int
}

type DescribeRestorableBranchesResult struct {
	Total          int             `json:"Total"`
	WorkspaceName  string          `json:"WorkspaceName"`
	Branches       []Branch        `json:"Branches"`
	RestoreWindows []RestoreWindow `json:"RestoreWindows"`
}

type BranchRestoreParams struct {
	WorkspaceID    string
	BranchID       string
	Time           string
	SourceBranchID string
}

type BranchRestoreResult struct {
	WorkspaceID    string `json:"WorkspaceId"`
	BranchID       string `json:"BranchId"`
	SourceBranchID string `json:"SourceBranchId,omitempty"`
	Time           string `json:"Time,omitempty"`
	BackupBranchID string `json:"BackupBranchId,omitempty"`
}

type ResetWorkspaceAccountPasswordParams struct {
	WorkspaceID     string
	BranchID        string
	AccountName     string
	AccountPassword string
}

type AccessControlListParams struct {
	WorkspaceID string
	Name        string
	AclType     string
	IPList      []string
}

type ModifyAccessControlListParams struct {
	WorkspaceID string
	Name        string
	ModifyMode  string
	IPList      []string
}

type DescribeAccessControlListResult struct {
	Total              int                 `json:"Total"`
	AccessControlLists []AccessControlList `json:"AccessControlLists"`
}

type AccessControlList struct {
	Name    string   `json:"Name"`
	AclType string   `json:"AclType"`
	IPList  []string `json:"IPList"`
}

type APIKey struct {
	Name       string `json:"Name"`
	Key        string `json:"Key"`
	Type       string `json:"Type"`
	CreateTime string `json:"CreateTime"`
}

type Branch struct {
	WorkspaceID  string `json:"WorkspaceId"`
	BranchID     string `json:"BranchId"`
	BranchName   string `json:"BranchName"`
	BranchStatus string `json:"BranchStatus"`
	Default      bool   `json:"Default"`
	Protected    bool   `json:"Protected"`
	Archived     bool   `json:"Archived"`
	InitSource   string `json:"InitSource"`
	CreateTime   string `json:"CreateTime"`
	UpdateTime   string `json:"UpdateTime"`
}

type BranchDetail struct {
	WorkspaceID       string       `json:"WorkspaceId"`
	BranchID          string       `json:"BranchId"`
	BranchName        string       `json:"BranchName"`
	BranchStatus      string       `json:"BranchStatus"`
	Default           bool         `json:"Default"`
	Protected         bool         `json:"Protected"`
	Archived          bool         `json:"Archived"`
	InitSource        string       `json:"InitSource"`
	CreationSource    string       `json:"CreationSource"`
	CreateTime        string       `json:"CreateTime"`
	UpdateTime        string       `json:"UpdateTime"`
	LastResetTime     string       `json:"LastResetTime"`
	StartParentLSN    string       `json:"StartParentLSN"`
	StartParentTime   string       `json:"StartParentTime"`
	StatusChangedTime string       `json:"StatusChangedTime"`
	ParentBranch      ParentBranch `json:"ParentBranch"`
	BranchUsage       BranchUsage  `json:"BranchUsage"`
}

type ParentBranch struct {
	WorkspaceID  string `json:"WorkspaceId"`
	BranchID     string `json:"BranchId"`
	BranchName   string `json:"BranchName"`
	BranchStatus string `json:"BranchStatus"`
}

type BranchUsage struct {
	WorkspaceID          string `json:"WorkspaceId"`
	BranchID             string `json:"BranchId"`
	ComputeTimeSeconds   int64  `json:"ComputeTimeSeconds"`
	DataSizeTotalBytes   int64  `json:"DataSizeTotalBytes"`
	DataSizeUsedBytes    int64  `json:"DataSizeUsedBytes"`
	FunctionCallNum      int64  `json:"FunctionCallNum"`
	LastRunningTime      string `json:"LastRunningTime"`
	ServiceTimeSeconds   int64  `json:"ServiceTimeSeconds"`
	StatTime             string `json:"StatTime"`
	StorageSizeUsedBytes int64  `json:"StorageSizeUsedBytes"`
}

func (c *Client) DescribeWorkspaceDetail(ctx context.Context, workspaceID string) (DescribeWorkspaceDetailResult, error) {
	resp, err := c.aidap.DescribeWorkspaceDetailWithContext(ctx, (&aidap.DescribeWorkspaceDetailInput{}).SetWorkspaceId(workspaceID))
	if err != nil {
		return DescribeWorkspaceDetailResult{}, errors.Errorf("failed to call volcengine DescribeWorkspaceDetail: %w", err)
	}
	return DescribeWorkspaceDetailResult{Workspace: mapWorkspaceFromDetail(resp.Workspace)}, nil
}

func (c *Client) DescribeDefaultBranch(ctx context.Context, workspaceID string) (DescribeDefaultBranchResult, error) {
	resp, err := c.aidap.DescribeDefaultBranchWithContext(ctx, (&aidap.DescribeDefaultBranchInput{}).SetWorkspaceId(workspaceID))
	if err != nil {
		return DescribeDefaultBranchResult{}, errors.Errorf("failed to call volcengine DescribeDefaultBranch: %w", err)
	}
	if resp == nil {
		return DescribeDefaultBranchResult{}, nil
	}
	return DescribeDefaultBranchResult{Branch: mapDefaultBranch(resp.Branch)}, nil
}

func (c *Client) DescribeWorkspaceEndpoints(ctx context.Context, params DescribeWorkspaceEndpointsParams) (DescribeWorkspaceEndpointsResult, error) {
	req := (&aidap.DescribeWorkspaceEndpointInput{}).SetWorkspaceId(params.WorkspaceID)
	if params.BranchID != "" {
		req.SetBranchId(params.BranchID)
	}
	resp, err := c.aidap.DescribeWorkspaceEndpointWithContext(ctx, req)
	if err != nil {
		return DescribeWorkspaceEndpointsResult{}, errors.Errorf("failed to call volcengine DescribeWorkspaceEndpoint: %w", err)
	}
	return mapDescribeWorkspaceEndpointsResult(resp), nil
}

func (c *Client) CreateEndpointPublicAddress(ctx context.Context, params CreateEndpointPublicAddressParams) error {
	req := (&aidap.CreateEndpointPublicAddressInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID)
	if params.EndpointID != "" {
		req.SetEndpointId(params.EndpointID)
	}
	if params.EIPID != "" {
		req.SetEipId(params.EIPID)
	}
	if _, err := c.aidap.CreateEndpointPublicAddressWithContext(ctx, req); err != nil {
		return errors.Errorf("failed to call volcengine CreateEndpointPublicAddress: %w", err)
	}
	return nil
}

func (c *Client) DeleteEndpointPublicAddress(ctx context.Context, params DeleteEndpointPublicAddressParams) error {
	req := (&aidap.DeleteEndpointPublicAddressInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID)
	if params.EndpointID != "" {
		req.SetEndpointId(params.EndpointID)
	}
	if _, err := c.aidap.DeleteEndpointPublicAddressWithContext(ctx, req); err != nil {
		return errors.Errorf("failed to call volcengine DeleteEndpointPublicAddress: %w", err)
	}
	return nil
}

func (c *Client) DescribeEIPAddress(ctx context.Context, allocationID string) (EIPAddress, error) {
	resp, err := c.vpc.DescribeEipAddressAttributesWithContext(ctx, (&vpc.DescribeEipAddressAttributesInput{}).SetAllocationId(allocationID))
	if err != nil {
		return EIPAddress{}, errors.Errorf("failed to call volcengine DescribeEipAddressAttributes: %w", err)
	}
	return mapEIPAddressAttributes(resp), nil
}

func (c *Client) DescribeAvailableEIPAddresses(ctx context.Context) ([]EIPAddress, error) {
	return c.DescribeEIPAddresses(ctx, DescribeEIPAddressesParams{
		AvailableOnly: true,
		Limit:         maxWorkspaceLimit,
	})
}

func (c *Client) DescribeEIPAddresses(ctx context.Context, params DescribeEIPAddressesParams) ([]EIPAddress, error) {
	var result []EIPAddress
	limit := normalizeVPCListLimit(params.Limit)
	req := (&vpc.DescribeEipAddressesInput{}).SetMaxResults(int64(limit))
	if params.AvailableOnly {
		req.SetStatus(vpc.StatusForDescribeEipAddressesInputAvailable)
	}
	resp, err := c.vpc.DescribeEipAddressesWithContext(ctx, req)
	if err != nil {
		return nil, errors.Errorf("failed to call volcengine DescribeEipAddresses: %w", err)
	}
	if resp == nil {
		return result, nil
	}
	for _, address := range resp.EipAddresses {
		result = append(result, mapEIPAddress(address))
	}
	return result, nil
}

func (c *Client) DescribeVPCs(ctx context.Context, limit int) ([]VPCNetwork, error) {
	var result []VPCNetwork
	req := (&vpc.DescribeVpcsInput{}).SetMaxResults(int64(normalizeVPCListLimit(limit)))
	resp, err := c.vpc.DescribeVpcsWithContext(ctx, req)
	if err != nil {
		return nil, errors.Errorf("failed to call volcengine DescribeVpcs: %w", err)
	}
	if resp == nil {
		return result, nil
	}
	for _, network := range resp.Vpcs {
		result = append(result, mapVPCNetwork(network))
	}
	return result, nil
}

func (c *Client) DescribeSubnets(ctx context.Context, vpcID string, limit int) ([]Subnet, error) {
	var result []Subnet
	req := (&vpc.DescribeSubnetsInput{}).
		SetMaxResults(int64(normalizeVPCListLimit(limit))).
		SetVpcId(vpcID)
	resp, err := c.vpc.DescribeSubnetsWithContext(ctx, req)
	if err != nil {
		return nil, errors.Errorf("failed to call volcengine DescribeSubnets: %w", err)
	}
	if resp == nil {
		return result, nil
	}
	for _, subnet := range resp.Subnets {
		result = append(result, mapSubnet(subnet))
	}
	return result, nil
}

func normalizeVPCListLimit(limit int) int {
	if limit <= 0 || limit > maxWorkspaceLimit {
		return maxWorkspaceLimit
	}
	return limit
}

func (c *Client) ModifyVpcSettings(ctx context.Context, params ModifyVpcSettingsParams) error {
	req := (&aidap.ModifyVpcSettingsInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetVpcId(params.VPCID).
		SetSubnetId(params.SubnetID).
		SetInternetProtocol(params.InternetProtocol)
	if params.BranchID != "" {
		req.SetBranchId(params.BranchID)
	}
	if _, err := c.aidap.ModifyVpcSettingsWithContext(ctx, req); err != nil {
		return errors.Errorf("failed to call volcengine ModifyVpcSettings: %w", err)
	}
	return nil
}

func (c *Client) DescribeAPIKeys(ctx context.Context, params DescribeAPIKeysParams) (DescribeAPIKeysResult, error) {
	limit := params.Limit
	if limit == 0 {
		limit = maxWorkspaceLimit
	}
	req := (&aidap.DescribeAPIKeysInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset))
	if params.BranchID != "" {
		req.SetBranchId(params.BranchID)
	}
	resp, err := c.aidap.DescribeAPIKeysWithContext(ctx, req)
	if err != nil {
		return DescribeAPIKeysResult{}, errors.Errorf("failed to call volcengine DescribeAPIKeys: %w", err)
	}
	return mapDescribeAPIKeysResult(resp), nil
}

func (c *Client) DescribeOperations(ctx context.Context, params DescribeOperationsParams) (DescribeOperationsResult, error) {
	limit := params.Limit
	if limit == 0 {
		limit = 10
	}
	req := (&aidap.DescribeOperationsInput{}).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset))
	if params.CreateTimeStart != "" {
		req.SetCreateTimeStart(params.CreateTimeStart)
	}
	if params.CreateTimeEnd != "" {
		req.SetCreateTimeEnd(params.CreateTimeEnd)
	}
	filters := make([]*aidap.FilterForDescribeOperationsInput, 0, 5)
	for _, filter := range []struct {
		name  string
		value string
	}{
		{name: "WorkspaceId", value: params.WorkspaceID},
		{name: "BranchId", value: params.BranchID},
		{name: "ComputeId", value: params.ComputeID},
		{name: "ActionName", value: params.ActionName},
		{name: "Status", value: params.Status},
	} {
		if filter.value != "" {
			filters = append(filters, (&aidap.FilterForDescribeOperationsInput{}).SetName(filter.name).SetValue(filter.value))
		}
	}
	if len(filters) > 0 {
		req.SetFilters(filters)
	}
	resp, err := c.aidap.DescribeOperationsWithContext(ctx, req)
	if err != nil {
		return DescribeOperationsResult{}, errors.Errorf("failed to call volcengine DescribeOperations: %w", err)
	}
	return mapDescribeOperationsResult(resp), nil
}

func (c *Client) DescribeBranches(ctx context.Context, params DescribeBranchesParams) (DescribeBranchesResult, error) {
	limit := params.Limit
	if limit == 0 {
		limit = maxWorkspaceLimit
	}
	req := (&aidap.DescribeBranchesInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetSortOrder(aidap.EnumOfSortOrderForDescribeBranchesInputDesc).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset))
	if params.Search != "" {
		req.SetSearch(params.Search)
	}
	resp, err := c.aidap.DescribeBranchesWithContext(ctx, req)
	if err != nil {
		return DescribeBranchesResult{}, errors.Errorf("failed to call volcengine DescribeBranches: %w", err)
	}
	return mapDescribeBranchesResult(resp), nil
}

func (c *Client) DescribeChildBranches(ctx context.Context, params DescribeChildBranchesParams) (DescribeBranchesResult, error) {
	limit := params.Limit
	if limit == 0 {
		limit = maxWorkspaceLimit
	}
	resp, err := c.aidap.DescribeChildBranchesWithContext(ctx, (&aidap.DescribeChildBranchesInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetParentBranchId(params.ParentID).
		SetSortOrder(aidap.EnumOfSortOrderForDescribeChildBranchesInputDesc).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset)))
	if err != nil {
		return DescribeBranchesResult{}, errors.Errorf("failed to call volcengine DescribeChildBranches: %w", err)
	}
	return mapDescribeChildBranchesResult(resp), nil
}

func (c *Client) DescribeBranchDetail(ctx context.Context, workspaceID, branchID string) (DescribeBranchDetailResult, error) {
	resp, err := c.aidap.DescribeBranchDetailWithContext(ctx, (&aidap.DescribeBranchDetailInput{}).
		SetWorkspaceId(workspaceID).
		SetBranchId(branchID))
	if err != nil {
		return DescribeBranchDetailResult{}, errors.Errorf("failed to call volcengine DescribeBranchDetail: %w", err)
	}
	return DescribeBranchDetailResult{
		WorkspaceName: stringValue(resp.WorkspaceName),
		Branch:        mapBranchDetail(resp.Branch),
	}, nil
}

func (c *Client) CreateBranch(ctx context.Context, params CreateBranchParams) (CreateBranchResult, error) {
	settings := (&aidap.BranchSettingsForCreateBranchInput{}).
		SetName(params.Name).
		SetInitSource(aidap.EnumOfInitSourceForCreateBranchInputParentData)
	if params.ParentID != "" {
		settings.SetParentId(params.ParentID)
	}
	if params.ParentTime != "" {
		settings.SetParentTime(params.ParentTime)
	}
	resp, err := c.aidap.CreateBranchWithContext(ctx, (&aidap.CreateBranchInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchSettings(settings))
	if err != nil {
		return CreateBranchResult{}, errors.Errorf("failed to call volcengine CreateBranch: %w", err)
	}
	return CreateBranchResult{
		WorkspaceID: stringValue(resp.WorkspaceId),
		BranchID:    stringValue(resp.BranchId),
		Branch:      mapCreatedBranchDetail(resp.Branch),
	}, nil
}

func (c *Client) UpdateBranch(ctx context.Context, params UpdateBranchParams) (UpdateBranchResult, error) {
	req := (&aidap.UpdateBranchInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID)
	if params.Name != nil {
		req.SetName(*params.Name)
	}
	resp, err := c.aidap.UpdateBranchWithContext(ctx, req)
	if err != nil {
		return UpdateBranchResult{}, errors.Errorf("failed to call volcengine UpdateBranch: %w", err)
	}
	return UpdateBranchResult{
		Branch: mapUpdatedBranchDetail(resp.Branch),
	}, nil
}

func (c *Client) DeleteBranch(ctx context.Context, workspaceID, branchID string) (DeleteBranchResult, error) {
	resp, err := c.aidap.DeleteBranchWithContext(ctx, (&aidap.DeleteBranchInput{}).
		SetWorkspaceId(workspaceID).
		SetBranchId(branchID))
	if err != nil {
		return DeleteBranchResult{}, errors.Errorf("failed to call volcengine DeleteBranch: %w", err)
	}
	return DeleteBranchResult{
		WorkspaceID: stringValue(resp.WorkspaceId),
		BranchID:    stringValue(resp.BranchId),
	}, nil
}

func (c *Client) RestartBranch(ctx context.Context, params RestartBranchParams) (RestartBranchResult, error) {
	req := (&aidap.RestartBranchInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID)
	if len(params.ComputeIDs) > 0 {
		computeIDs := make([]*string, len(params.ComputeIDs))
		for i := range params.ComputeIDs {
			computeIDs[i] = &params.ComputeIDs[i]
		}
		req.SetComputeIds(computeIDs)
	}
	if _, err := c.aidap.RestartBranchWithContext(ctx, req); err != nil {
		return RestartBranchResult{}, errors.Errorf("failed to call volcengine RestartBranch: %w", err)
	}
	return RestartBranchResult{
		WorkspaceID: params.WorkspaceID,
		BranchID:    params.BranchID,
		ComputeIDs:  append([]string(nil), params.ComputeIDs...),
	}, nil
}

func (c *Client) SetAsDefaultBranch(ctx context.Context, workspaceID, branchID string) (SetAsDefaultBranchResult, error) {
	resp, err := c.aidap.SetAsDefaultBranchWithContext(ctx, (&aidap.SetAsDefaultBranchInput{}).
		SetWorkspaceId(workspaceID).
		SetBranchId(branchID))
	if err != nil {
		return SetAsDefaultBranchResult{}, errors.Errorf("failed to call volcengine SetAsDefaultBranch: %w", err)
	}
	return SetAsDefaultBranchResult{
		Branch: mapSetAsDefaultBranchDetail(resp.Branch),
	}, nil
}

func (c *Client) GetRestoreWindow(ctx context.Context, workspaceID, branchID string) (RestoreWindow, error) {
	resp, err := c.aidap.GetRestoreWindowWithContext(ctx, (&aidap.GetRestoreWindowInput{}).
		SetWorkspaceId(workspaceID).
		SetBranchId(branchID))
	if err != nil {
		return RestoreWindow{}, errors.Errorf("failed to call volcengine GetRestoreWindow: %w", err)
	}
	return mapRestoreWindow(resp), nil
}

func (c *Client) DescribeRestorableBranches(ctx context.Context, params DescribeRestorableBranchesParams) (DescribeRestorableBranchesResult, error) {
	limit := params.Limit
	if limit == 0 {
		limit = maxWorkspaceLimit
	}
	req := (&aidap.DescribeRestorableBranchesInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetTime(params.Time).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset))
	if params.Search != "" {
		req.SetSearch(params.Search)
	}
	resp, err := c.aidap.DescribeRestorableBranchesWithContext(ctx, req)
	if err != nil {
		return DescribeRestorableBranchesResult{}, errors.Errorf("failed to call volcengine DescribeRestorableBranches: %w", err)
	}
	return mapDescribeRestorableBranchesResult(resp), nil
}

func (c *Client) BranchRestore(ctx context.Context, params BranchRestoreParams) (BranchRestoreResult, error) {
	settings := &aidap.RestoreSettingsForBranchRestoreInput{}
	if params.Time != "" {
		settings.SetTime(params.Time)
	}
	if params.SourceBranchID != "" {
		settings.SetSourceBranchId(params.SourceBranchID)
	}
	resp, err := c.aidap.BranchRestoreWithContext(ctx, (&aidap.BranchRestoreInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetRestoreSettings(settings))
	if err != nil {
		return BranchRestoreResult{}, errors.Errorf("failed to call volcengine BranchRestore: %w", err)
	}
	return BranchRestoreResult{
		WorkspaceID:    params.WorkspaceID,
		BranchID:       params.BranchID,
		SourceBranchID: params.SourceBranchID,
		Time:           params.Time,
		BackupBranchID: stringValue(resp.BackupBranchID),
	}, nil
}

func (c *Client) ResetWorkspaceAccountPassword(ctx context.Context, params ResetWorkspaceAccountPasswordParams) error {
	req := (&aidap.ResetWorkspaceAccountPasswordInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetAccountName(params.AccountName).
		SetAccountPassword(params.AccountPassword)
	if _, err := c.aidap.ResetWorkspaceAccountPasswordWithContext(ctx, req); err != nil {
		return errors.Errorf("failed to call volcengine ResetWorkspaceAccountPassword: %w", err)
	}
	return nil
}

func (c *Client) CreateAccessControlList(ctx context.Context, params AccessControlListParams) error {
	_, err := c.aidap.CreateAccessControlListWithContext(ctx, (&aidap.CreateAccessControlListInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetAccessControlListName(params.Name).
		SetAclType(params.AclType).
		SetIPList(stringSlicePointers(params.IPList)))
	if err != nil {
		return errors.Errorf("failed to call volcengine CreateAccessControlList: %w", err)
	}
	return nil
}

func (c *Client) DescribeAccessControlList(ctx context.Context, workspaceID, name string) (DescribeAccessControlListResult, error) {
	req := (&aidap.DescribeAccessControlListInput{}).SetWorkspaceId(workspaceID)
	if name != "" {
		req.SetAccessControlListName(name)
	}
	resp, err := c.aidap.DescribeAccessControlListWithContext(ctx, req)
	if err != nil {
		return DescribeAccessControlListResult{}, errors.Errorf("failed to call volcengine DescribeAccessControlList: %w", err)
	}
	return mapDescribeAccessControlListResult(resp), nil
}

func (c *Client) ModifyAccessControlList(ctx context.Context, params ModifyAccessControlListParams) error {
	_, err := c.aidap.ModifyAccessControlListWithContext(ctx, (&aidap.ModifyAccessControlListInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetAccessControlListName(params.Name).
		SetModifyMode(params.ModifyMode).
		SetIPList(stringSlicePointers(params.IPList)))
	if err != nil {
		return errors.Errorf("failed to call volcengine ModifyAccessControlList: %w", err)
	}
	return nil
}

func (c *Client) DeleteAccessControlList(ctx context.Context, workspaceID, name string) error {
	_, err := c.aidap.DeleteAccessControlListWithContext(ctx, (&aidap.DeleteAccessControlListInput{}).
		SetWorkspaceId(workspaceID).
		SetAccessControlListName(name))
	if err != nil {
		return errors.Errorf("failed to call volcengine DeleteAccessControlList: %w", err)
	}
	return nil
}

func (c *Client) DescribeAllBranches(ctx context.Context, params DescribeBranchesParams) (DescribeBranchesResult, error) {
	limit := params.Limit
	if limit == 0 || limit > maxWorkspaceLimit {
		limit = maxWorkspaceLimit
	}
	params.Limit = limit

	var result DescribeBranchesResult
	for {
		page, err := c.DescribeBranches(ctx, params)
		if err != nil {
			return DescribeBranchesResult{}, err
		}
		result.Total = page.Total
		if result.WorkspaceName == "" {
			result.WorkspaceName = page.WorkspaceName
		}
		result.Branches = append(result.Branches, page.Branches...)
		if len(result.Branches) >= page.Total || len(page.Branches) == 0 {
			return result, nil
		}
		params.Offset += limit
	}
}

func (c *Client) DescribeAllChildBranches(ctx context.Context, params DescribeChildBranchesParams) (DescribeBranchesResult, error) {
	limit := params.Limit
	if limit == 0 || limit > maxWorkspaceLimit {
		limit = maxWorkspaceLimit
	}
	params.Limit = limit

	var result DescribeBranchesResult
	for {
		page, err := c.DescribeChildBranches(ctx, params)
		if err != nil {
			return DescribeBranchesResult{}, err
		}
		result.Total = page.Total
		if result.WorkspaceName == "" {
			result.WorkspaceName = page.WorkspaceName
		}
		result.Branches = append(result.Branches, page.Branches...)
		if len(result.Branches) >= page.Total || len(page.Branches) == 0 {
			return result, nil
		}
		params.Offset += limit
	}
}

type DescribeComputesResult struct {
	Total    int
	Computes []Compute
}

type Compute struct {
	WorkspaceID           string  `json:"WorkspaceId"`
	BranchID              string  `json:"BranchId"`
	ComputeID             string  `json:"ComputeId"`
	ComputeName           string  `json:"ComputeName"`
	ComputeStatus         string  `json:"ComputeStatus"`
	ComputeRole           string  `json:"ComputeRole"`
	ServiceType           string  `json:"ServiceType"`
	AutoScalingLimitMinCU float64 `json:"AutoScalingLimitMinCU"`
	AutoScalingLimitMaxCU float64 `json:"AutoScalingLimitMaxCU"`
	EnableAnalytics       string  `json:"EnableAnalytics"`
	CreationSource        string  `json:"CreationSource"`
	Disabled              bool    `json:"Disabled"`
	CreateTime            string  `json:"CreateTime"`
	UpdateTime            string  `json:"UpdateTime"`
	LastActiveTime        string  `json:"LastActiveTime"`
	StatusChangedTime     string  `json:"StatusChangedTime"`
	SuspendedTime         string  `json:"SuspendedTime"`
}

type ModifyComputeSpecParams struct {
	WorkspaceID           string
	ComputeID             string
	AutoScalingLimitMinCU float64
	AutoScalingLimitMaxCU float64
}

type ModifyComputeNameParams struct {
	WorkspaceID string
	ComputeID   string
	ComputeName string
}

type DescribeDBAccountConnectionParams struct {
	WorkspaceID  string
	BranchID     string
	ComputeID    string
	AccountName  string
	DatabaseName string
}

type DBAccountConnection struct {
	WorkspaceID            string
	BranchID               string
	ComputeID              string
	AccountName            string
	AccountPassword        string
	AllowHost              string
	DatabaseName           string
	ConnectionURL          string
	ConnectionExampleCount int
}

func (c *Client) DescribeComputes(ctx context.Context, workspaceID, branchID, serviceType string) (DescribeComputesResult, error) {
	req := (&aidap.DescribeComputesInput{}).
		SetWorkspaceId(workspaceID).
		SetBranchId(branchID)
	if serviceType != "" {
		req.SetServiceType(serviceType)
	}
	resp, err := c.aidap.DescribeComputesWithContext(ctx, req)
	if err != nil {
		return DescribeComputesResult{}, errors.Errorf("failed to call volcengine DescribeComputes: %w", err)
	}
	return mapDescribeComputesResult(resp), nil
}

// DescribeBranchComputes lists a branch's computes. When serviceType is empty it
// queries both the Database and Supabase service types and de-duplicates by
// compute id. Single source of truth shared by the CLI computes commands and the
// MCP compute tools.
func (c *Client) DescribeBranchComputes(ctx context.Context, workspaceID, branchID, serviceType string) (DescribeComputesResult, error) {
	serviceTypes := []string{strings.TrimSpace(serviceType)}
	if serviceTypes[0] == "" {
		serviceTypes = []string{ServiceTypeDatabase, ServiceTypeSupabase}
	}
	var merged DescribeComputesResult
	seen := map[string]struct{}{}
	for _, st := range serviceTypes {
		result, err := c.DescribeComputes(ctx, workspaceID, branchID, st)
		if err != nil {
			return DescribeComputesResult{}, err
		}
		for _, compute := range result.Computes {
			if compute.ComputeID == "" {
				continue
			}
			if _, ok := seen[compute.ComputeID]; ok {
				continue
			}
			seen[compute.ComputeID] = struct{}{}
			merged.Computes = append(merged.Computes, compute)
		}
	}
	merged.Total = len(merged.Computes)
	return merged, nil
}

func (c *Client) DescribeComputeDetail(ctx context.Context, workspaceID, computeID string) (Compute, error) {
	resp, err := c.aidap.DescribeComputeDetailWithContext(ctx, (&aidap.DescribeComputeDetailInput{}).
		SetWorkspaceId(workspaceID).
		SetComputeId(computeID))
	if err != nil {
		return Compute{}, errors.Errorf("failed to call volcengine DescribeComputeDetail: %w", err)
	}
	if resp == nil {
		return Compute{}, nil
	}
	return mapComputeDetail(resp.Compute), nil
}

func (c *Client) DescribeDBAccountConnection(ctx context.Context, params DescribeDBAccountConnectionParams) (DBAccountConnection, error) {
	resp, err := c.aidap.DescribeDBAccountConnectionWithContext(ctx, (&aidap.DescribeDBAccountConnectionInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetComputeId(params.ComputeID).
		SetAccountName(params.AccountName).
		SetDatabaseName(params.DatabaseName))
	if err != nil {
		return DBAccountConnection{}, errors.Errorf("failed to call volcengine DescribeDBAccountConnection: %w", err)
	}
	if resp == nil {
		return DBAccountConnection{}, nil
	}
	return DBAccountConnection{
		WorkspaceID:            stringValue(resp.WorkspaceId),
		BranchID:               stringValue(resp.BranchId),
		ComputeID:              stringValue(resp.ComputeId),
		AccountName:            stringValue(resp.AccountName),
		AccountPassword:        stringValue(resp.AccountPassword),
		AllowHost:              stringValue(resp.AllowHost),
		DatabaseName:           stringValue(resp.DatabaseName),
		ConnectionURL:          stringValue(resp.ConnectionUrl),
		ConnectionExampleCount: len(resp.ConnectionExamples),
	}, nil
}

// ResolvePrimaryDatabaseComputeID resolves the primary Database compute of a
// branch. When branchID is empty the workspace's default branch is used. It is
// the single source of truth for the "resolve branch → find primary database
// compute" path shared by the CLI db connection helpers and the MCP
// get_db_account_connection tool.
func (c *Client) ResolvePrimaryDatabaseComputeID(ctx context.Context, workspaceID, branchID string) (resolvedBranchID, computeID string, err error) {
	branchID, err = c.ResolveDefaultBranchID(ctx, workspaceID, branchID)
	if err != nil {
		return "", "", err
	}
	computes, err := c.DescribeComputes(ctx, workspaceID, branchID, ServiceTypeDatabase)
	if err != nil {
		return "", "", err
	}
	for _, compute := range computes.Computes {
		if strings.EqualFold(compute.ServiceType, ServiceTypeDatabase) &&
			strings.EqualFold(compute.ComputeRole, ComputeRolePrimary) {
			return branchID, compute.ComputeID, nil
		}
	}
	return "", "", errors.Errorf("primary database compute not found for branch %s", branchID)
}

// Database is a logical database inside a branch.
type Database struct {
	WorkspaceID   string
	BranchID      string
	DatabaseName  string
	DatabaseOwner string
	DatabaseDesc  string
	CreateTime    string
	UpdateTime    string
}

type DescribeDatabasesParams struct {
	WorkspaceID string
	BranchID    string
	Search      string
	Limit       int
	Offset      int
}

type DescribeDatabasesResult struct {
	Databases []Database
	Total     int
}

func (c *Client) DescribeDatabases(ctx context.Context, params DescribeDatabasesParams) (DescribeDatabasesResult, error) {
	// The backend defaults to 10 per page when Limit is not supplied; pass Limit/Offset
	// explicitly here: when the caller omits count (Limit<=0) fall back to
	// DefaultListLimit(10) to stay consistent with that convention.
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	req := (&aidap.DescribeDatabasesInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset))
	if params.Search != "" {
		req.SetSearch(params.Search)
	}
	resp, err := c.aidap.DescribeDatabasesWithContext(ctx, req)
	if err != nil {
		return DescribeDatabasesResult{}, errors.Errorf("failed to call volcengine DescribeDatabases: %w", err)
	}
	if resp == nil {
		return DescribeDatabasesResult{}, nil
	}
	result := DescribeDatabasesResult{Total: intValue(resp.Total)}
	for _, db := range resp.Databases {
		if db == nil {
			continue
		}
		result.Databases = append(result.Databases, Database{
			WorkspaceID:   stringValue(db.WorkspaceId),
			BranchID:      stringValue(db.BranchId),
			DatabaseName:  stringValue(db.DatabaseName),
			DatabaseOwner: stringValue(db.DatabaseOwner),
			DatabaseDesc:  stringValue(db.DatabaseDesc),
			CreateTime:    stringValue(db.CreateTime),
			UpdateTime:    stringValue(db.UpdateTime),
		})
	}
	return result, nil
}

type CreateDatabaseParams struct {
	WorkspaceID  string
	BranchID     string
	DatabaseName string
	Owner        string
	Description  string
}

func (c *Client) CreateDatabase(ctx context.Context, params CreateDatabaseParams) (Database, error) {
	req := (&aidap.CreateDatabaseInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetDatabaseName(params.DatabaseName)
	if params.Owner != "" {
		req.SetDatabaseOwner(params.Owner)
	}
	if params.Description != "" {
		req.SetDatabaseDesc(params.Description)
	}
	resp, err := c.aidap.CreateDatabaseWithContext(ctx, req)
	if err != nil {
		return Database{}, errors.Errorf("failed to call volcengine CreateDatabase: %w", err)
	}
	if resp == nil || resp.Database == nil {
		return Database{WorkspaceID: params.WorkspaceID, BranchID: params.BranchID, DatabaseName: params.DatabaseName}, nil
	}
	db := resp.Database
	return Database{
		WorkspaceID:   stringValue(db.WorkspaceId),
		BranchID:      stringValue(db.BranchId),
		DatabaseName:  stringValue(db.DatabaseName),
		DatabaseOwner: stringValue(db.DatabaseOwner),
		DatabaseDesc:  stringValue(db.DatabaseDesc),
		CreateTime:    stringValue(db.CreateTime),
		UpdateTime:    stringValue(db.UpdateTime),
	}, nil
}

// DBAccount is a Postgres role/account inside a branch.
type DBAccount struct {
	WorkspaceID string
	BranchID    string
	AccountName string
	AccountDesc string
	CreateTime  string
	UpdateTime  string
}

type DescribeDBAccountsParams struct {
	WorkspaceID string
	BranchID    string
	Search      string
	Limit       int
	Offset      int
}

type DescribeDBAccountsResult struct {
	Accounts []DBAccount
	Total    int
}

func (c *Client) DescribeDBAccounts(ctx context.Context, params DescribeDBAccountsParams) (DescribeDBAccountsResult, error) {
	// Same as DescribeDatabases: the backend defaults to 10 per page when Limit is omitted;
	// pass Limit/Offset explicitly and fall back to DefaultListLimit(10) when the caller
	// omits count (Limit<=0).
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	req := (&aidap.DescribeDBAccountsInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetLimit(int32(limit)).
		SetOffset(int32(params.Offset))
	if params.Search != "" {
		req.SetSearch(params.Search)
	}
	resp, err := c.aidap.DescribeDBAccountsWithContext(ctx, req)
	if err != nil {
		return DescribeDBAccountsResult{}, errors.Errorf("failed to call volcengine DescribeDBAccounts: %w", err)
	}
	if resp == nil {
		return DescribeDBAccountsResult{}, nil
	}
	result := DescribeDBAccountsResult{Total: intValue(resp.Total)}
	for _, acct := range resp.Accounts {
		if acct == nil {
			continue
		}
		result.Accounts = append(result.Accounts, DBAccount{
			WorkspaceID: stringValue(acct.WorkspaceId),
			BranchID:    stringValue(acct.BranchId),
			AccountName: stringValue(acct.AccountName),
			AccountDesc: stringValue(acct.AccountDesc),
			CreateTime:  stringValue(acct.CreateTime),
			UpdateTime:  stringValue(acct.UpdateTime),
		})
	}
	return result, nil
}

type CreateDBAccountParams struct {
	WorkspaceID     string
	BranchID        string
	AccountName     string
	AccountPassword string
	Description     string
}

func (c *Client) CreateDBAccount(ctx context.Context, params CreateDBAccountParams) (DBAccount, error) {
	req := (&aidap.CreateDBAccountInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetBranchId(params.BranchID).
		SetAccountName(params.AccountName).
		SetAccountPassword(params.AccountPassword)
	if params.Description != "" {
		req.SetAccountDesc(params.Description)
	}
	resp, err := c.aidap.CreateDBAccountWithContext(ctx, req)
	if err != nil {
		return DBAccount{}, errors.Errorf("failed to call volcengine CreateDBAccount: %w", err)
	}
	if resp == nil || resp.Account == nil {
		return DBAccount{WorkspaceID: params.WorkspaceID, BranchID: params.BranchID, AccountName: params.AccountName}, nil
	}
	acct := resp.Account
	return DBAccount{
		WorkspaceID: stringValue(acct.WorkspaceId),
		BranchID:    stringValue(acct.BranchId),
		AccountName: stringValue(acct.AccountName),
		AccountDesc: stringValue(acct.AccountDesc),
		CreateTime:  stringValue(acct.CreateTime),
		UpdateTime:  stringValue(acct.UpdateTime),
	}, nil
}

func (c *Client) ModifyComputeSpec(ctx context.Context, params ModifyComputeSpecParams) (Compute, error) {
	resp, err := c.aidap.ModifyComputeSpecWithContext(ctx, (&aidap.ModifyComputeSpecInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetComputeId(params.ComputeID).
		SetAutoScalingLimitMinCU(params.AutoScalingLimitMinCU).
		SetAutoScalingLimitMaxCU(params.AutoScalingLimitMaxCU))
	if err != nil {
		return Compute{}, errors.Errorf("failed to call volcengine ModifyComputeSpec: %w", err)
	}
	if resp == nil {
		return Compute{}, nil
	}
	return mapModifiedCompute(resp.Compute), nil
}

func (c *Client) ModifyComputeName(ctx context.Context, params ModifyComputeNameParams) error {
	_, err := c.aidap.ModifyComputeNameWithContext(ctx, (&aidap.ModifyComputeNameInput{}).
		SetWorkspaceId(params.WorkspaceID).
		SetComputeId(params.ComputeID).
		SetComputeName(params.ComputeName))
	if err != nil {
		return errors.Errorf("failed to call volcengine ModifyComputeName: %w", err)
	}
	return nil
}

func mapWorkspaceFromDetail(workspace *aidap.WorkspaceForDescribeWorkspaceDetailOutput) Workspace {
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
	result.CreationSource = stringValue(workspace.CreationSource)
	result.DeletionProtectionStatus = stringValue(workspace.DeletionProtectionStatus)
	result.InternetProtocol = stringValue(workspace.InternetProtocol)
	result.DNSVisibility = boolValue(workspace.DNSVisibility)
	result.SharedPrivateNetwork = boolValue(workspace.SharedPrivateNetwork)
	result.IsAgentPlan = boolValue(workspace.IsAgentPlan)
	result.IsAgentPlanInstance = boolValue(workspace.IsAgentPlanInstance)
	result.AgentPlanSeatID = stringValue(workspace.AgentPlanSeatId)
	result.VpcID = stringValue(workspace.VpcId)
	result.SubnetID = stringValue(workspace.SubnetId)
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
	// SuspendTimeoutSeconds is surfaced on BaasComputeSettings (the effective compute
	// config), not the plain ComputeSettings which omits it, so prefer it when present.
	if workspace.BaasComputeSettings != nil && workspace.BaasComputeSettings.SuspendTimeoutSeconds != nil {
		result.ComputeSettings.SuspendTimeoutSeconds = intValue(workspace.BaasComputeSettings.SuspendTimeoutSeconds)
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
	for _, tag := range workspace.WorkspaceTags {
		if tag == nil {
			continue
		}
		result.WorkspaceTags = append(result.WorkspaceTags, WorkspaceTag{
			Key:    stringValue(tag.Key),
			Value:  stringValue(tag.Value),
			System: boolValue(tag.System),
		})
	}
	return result
}

func mapDescribeAPIKeysResult(resp *aidap.DescribeAPIKeysOutput) DescribeAPIKeysResult {
	if resp == nil {
		return DescribeAPIKeysResult{}
	}
	result := DescribeAPIKeysResult{
		Total: intValue(resp.Total),
	}
	for _, apiKey := range resp.APIKeys {
		result.APIKeys = append(result.APIKeys, mapAPIKey(apiKey))
	}
	return result
}

func mapDescribeOperationsResult(resp *aidap.DescribeOperationsOutput) DescribeOperationsResult {
	if resp == nil {
		return DescribeOperationsResult{}
	}
	result := DescribeOperationsResult{
		Total: intValue(resp.Total),
	}
	for _, operation := range resp.Operations {
		if operation == nil {
			continue
		}
		result.Operations = append(result.Operations, Operation{
			OperationID:  stringValue(operation.OperationId),
			WorkspaceID:  stringValue(operation.WorkspaceId),
			BranchID:     stringValue(operation.BranchId),
			ComputeID:    stringValue(operation.ComputeId),
			ActionName:   stringValue(operation.ActionName),
			ActionStatus: stringValue(operation.ActionStatus),
			CreateTime:   stringValue(operation.CreateTime),
			FinishTime:   stringValue(operation.FinishTime),
			DurationTime: stringValue(operation.DurationTime),
		})
	}
	return result
}

func mapDefaultBranch(branch *aidap.BranchForDescribeDefaultBranchOutput) Branch {
	if branch == nil {
		return Branch{}
	}
	return Branch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
		Default:      boolValue(branch.Default),
		Protected:    boolValue(branch.Protected),
		Archived:     boolValue(branch.Archived),
		InitSource:   stringValue(branch.InitSource),
		CreateTime:   stringValue(branch.CreateTime),
		UpdateTime:   stringValue(branch.UpdateTime),
	}
}

func mapDescribeWorkspaceEndpointsResult(resp *aidap.DescribeWorkspaceEndpointOutput) DescribeWorkspaceEndpointsResult {
	if resp == nil {
		return DescribeWorkspaceEndpointsResult{}
	}
	result := DescribeWorkspaceEndpointsResult{
		WorkspaceID: stringValue(resp.WorkspaceId),
		BranchID:    stringValue(resp.BranchId),
	}
	for _, endpoint := range resp.Endpoints {
		if endpoint == nil {
			continue
		}
		item := Endpoint{
			EndpointID:   stringValue(endpoint.EndpointId),
			EndpointName: stringValue(endpoint.EndpointName),
			EndpointType: stringValue(endpoint.EndpointType),
		}
		for _, address := range endpoint.Addresses {
			if address == nil {
				continue
			}
			item.Addresses = append(item.Addresses, EndpointAddress{
				AddressID:     stringValue(address.AddressId),
				AddressType:   stringValue(address.AddressType),
				AddressDomain: stringValue(address.AddressDomain),
				AddressPort:   intValue(address.AddressPort),
				IPAddress:     stringValue(address.IPAddress),
				IPv6Address:   stringValue(address.IPv6Address),
			})
		}
		result.Endpoints = append(result.Endpoints, item)
	}
	return result
}

func mapEIPAddressAttributes(address *vpc.DescribeEipAddressAttributesOutput) EIPAddress {
	if address == nil {
		return EIPAddress{}
	}
	return EIPAddress{
		AllocationID: stringValue(address.AllocationId),
		Address:      stringValue(address.EipAddress),
		Name:         stringValue(address.Name),
		Status:       stringValue(address.Status),
		InstanceID:   stringValue(address.InstanceId),
		InstanceType: stringValue(address.InstanceType),
		Bandwidth:    int64Value(address.Bandwidth),
		ISP:          stringValue(address.ISP),
	}
}

func mapEIPAddress(address *vpc.EipAddressForDescribeEipAddressesOutput) EIPAddress {
	if address == nil {
		return EIPAddress{}
	}
	return EIPAddress{
		AllocationID: stringValue(address.AllocationId),
		Address:      stringValue(address.EipAddress),
		Name:         stringValue(address.Name),
		Status:       stringValue(address.Status),
		InstanceID:   stringValue(address.InstanceId),
		InstanceType: stringValue(address.InstanceType),
		Bandwidth:    int64Value(address.Bandwidth),
		ISP:          stringValue(address.ISP),
	}
}

func mapVPCNetwork(network *vpc.VpcForDescribeVpcsOutput) VPCNetwork {
	if network == nil {
		return VPCNetwork{}
	}
	return VPCNetwork{
		VPCID:         stringValue(network.VpcId),
		VPCName:       stringValue(network.VpcName),
		CIDRBlock:     stringValue(network.CidrBlock),
		IPv6CIDRBlock: stringValue(network.Ipv6CidrBlock),
		Status:        stringValue(network.Status),
		ProjectName:   stringValue(network.ProjectName),
		Default:       boolValue(network.IsDefault),
		SubnetIDs:     stringPointersValue(network.SubnetIds),
	}
}

func mapSubnet(subnet *vpc.SubnetForDescribeSubnetsOutput) Subnet {
	if subnet == nil {
		return Subnet{}
	}
	return Subnet{
		SubnetID:                stringValue(subnet.SubnetId),
		SubnetName:              stringValue(subnet.SubnetName),
		VPCID:                   stringValue(subnet.VpcId),
		CIDRBlock:               stringValue(subnet.CidrBlock),
		IPv6CIDRBlock:           stringValue(subnet.Ipv6CidrBlock),
		Status:                  stringValue(subnet.Status),
		ZoneID:                  stringValue(subnet.ZoneId),
		AvailableIPAddressCount: int64Value(subnet.AvailableIpAddressCount),
		Default:                 boolValue(subnet.IsDefault),
	}
}

func mapDescribeBranchesResult(resp *aidap.DescribeBranchesOutput) DescribeBranchesResult {
	if resp == nil {
		return DescribeBranchesResult{}
	}
	result := DescribeBranchesResult{
		Total:         intValue(resp.Total),
		WorkspaceName: stringValue(resp.WorkspaceName),
	}
	for _, branch := range resp.Branches {
		result.Branches = append(result.Branches, mapBranch(branch))
	}
	return result
}

func mapDescribeChildBranchesResult(resp *aidap.DescribeChildBranchesOutput) DescribeBranchesResult {
	if resp == nil {
		return DescribeBranchesResult{}
	}
	result := DescribeBranchesResult{
		Total:         intValue(resp.Total),
		WorkspaceName: stringValue(resp.WorkspaceName),
	}
	for _, branch := range resp.Branches {
		result.Branches = append(result.Branches, mapChildBranch(branch))
	}
	return result
}

func mapBranch(branch *aidap.BranchForDescribeBranchesOutput) Branch {
	if branch == nil {
		return Branch{}
	}
	return Branch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
		Default:      boolValue(branch.Default),
		Protected:    boolValue(branch.Protected),
		Archived:     boolValue(branch.Archived),
		InitSource:   stringValue(branch.InitSource),
		CreateTime:   stringValue(branch.CreateTime),
		UpdateTime:   stringValue(branch.UpdateTime),
	}
}

func mapChildBranch(branch *aidap.BranchForDescribeChildBranchesOutput) Branch {
	if branch == nil {
		return Branch{}
	}
	return Branch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
		Default:      boolValue(branch.Default),
		Protected:    boolValue(branch.Protected),
		Archived:     boolValue(branch.Archived),
		InitSource:   stringValue(branch.InitSource),
		CreateTime:   stringValue(branch.CreateTime),
		UpdateTime:   stringValue(branch.UpdateTime),
	}
}

func mapBranchDetail(branch *aidap.BranchForDescribeBranchDetailOutput) BranchDetail {
	if branch == nil {
		return BranchDetail{}
	}
	return BranchDetail{
		WorkspaceID:       stringValue(branch.WorkspaceId),
		BranchID:          stringValue(branch.BranchId),
		BranchName:        stringValue(branch.BranchName),
		BranchStatus:      stringValue(branch.BranchStatus),
		Default:           boolValue(branch.Default),
		Protected:         boolValue(branch.Protected),
		Archived:          boolValue(branch.Archived),
		InitSource:        stringValue(branch.InitSource),
		CreationSource:    stringValue(branch.CreationSource),
		CreateTime:        stringValue(branch.CreateTime),
		UpdateTime:        stringValue(branch.UpdateTime),
		LastResetTime:     stringValue(branch.LastResetTime),
		StartParentLSN:    stringValue(branch.StartParentLSN),
		StartParentTime:   stringValue(branch.StartParentTime),
		StatusChangedTime: stringValue(branch.StatusChangedTime),
		ParentBranch:      mapParentBranch(branch.ParentBranch),
		BranchUsage:       mapBranchUsage(branch.BranchUsage),
	}
}

func mapParentBranch(branch *aidap.ParentBranchForDescribeBranchDetailOutput) ParentBranch {
	if branch == nil {
		return ParentBranch{}
	}
	return ParentBranch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
	}
}

func mapBranchUsage(usage *aidap.BranchUsageForDescribeBranchDetailOutput) BranchUsage {
	if usage == nil {
		return BranchUsage{}
	}
	return BranchUsage{
		WorkspaceID:          stringValue(usage.WorkspaceId),
		BranchID:             stringValue(usage.BranchId),
		ComputeTimeSeconds:   int64Value(usage.ComputeTimeSeconds),
		DataSizeTotalBytes:   int64Value(usage.DataSizeTotalBytes),
		DataSizeUsedBytes:    int64Value(usage.DataSizeUsedBytes),
		FunctionCallNum:      int64Value(usage.FunctionCallNum),
		LastRunningTime:      stringValue(usage.LastRunningTime),
		ServiceTimeSeconds:   int64Value(usage.ServiceTimeSeconds),
		StatTime:             stringValue(usage.StatTime),
		StorageSizeUsedBytes: int64Value(usage.StorageSizeUsedBytes),
	}
}

func mapCreatedBranchDetail(branch *aidap.BranchForCreateBranchOutput) BranchDetail {
	if branch == nil {
		return BranchDetail{}
	}
	return BranchDetail{
		WorkspaceID:       stringValue(branch.WorkspaceId),
		BranchID:          stringValue(branch.BranchId),
		BranchName:        stringValue(branch.BranchName),
		BranchStatus:      stringValue(branch.BranchStatus),
		Default:           boolValue(branch.Default),
		Protected:         boolValue(branch.Protected),
		Archived:          boolValue(branch.Archived),
		InitSource:        stringValue(branch.InitSource),
		CreationSource:    stringValue(branch.CreationSource),
		CreateTime:        stringValue(branch.CreateTime),
		UpdateTime:        stringValue(branch.UpdateTime),
		LastResetTime:     stringValue(branch.LastResetTime),
		StartParentLSN:    stringValue(branch.StartParentLSN),
		StartParentTime:   stringValue(branch.StartParentTime),
		StatusChangedTime: stringValue(branch.StatusChangedTime),
		ParentBranch:      mapCreatedParentBranch(branch.ParentBranch),
		BranchUsage:       mapCreatedBranchUsage(branch.BranchUsage),
	}
}

func mapCreatedParentBranch(branch *aidap.ParentBranchForCreateBranchOutput) ParentBranch {
	if branch == nil {
		return ParentBranch{}
	}
	return ParentBranch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
	}
}

func mapCreatedBranchUsage(usage *aidap.BranchUsageForCreateBranchOutput) BranchUsage {
	if usage == nil {
		return BranchUsage{}
	}
	return BranchUsage{
		WorkspaceID:          stringValue(usage.WorkspaceId),
		BranchID:             stringValue(usage.BranchId),
		ComputeTimeSeconds:   int64Value(usage.ComputeTimeSeconds),
		DataSizeTotalBytes:   int64Value(usage.DataSizeTotalBytes),
		DataSizeUsedBytes:    int64Value(usage.DataSizeUsedBytes),
		FunctionCallNum:      int64Value(usage.FunctionCallNum),
		LastRunningTime:      stringValue(usage.LastRunningTime),
		ServiceTimeSeconds:   int64Value(usage.ServiceTimeSeconds),
		StatTime:             stringValue(usage.StatTime),
		StorageSizeUsedBytes: int64Value(usage.StorageSizeUsedBytes),
	}
}

func mapUpdatedBranchDetail(branch *aidap.BranchForUpdateBranchOutput) BranchDetail {
	if branch == nil {
		return BranchDetail{}
	}
	return BranchDetail{
		WorkspaceID:       stringValue(branch.WorkspaceId),
		BranchID:          stringValue(branch.BranchId),
		BranchName:        stringValue(branch.BranchName),
		BranchStatus:      stringValue(branch.BranchStatus),
		Default:           boolValue(branch.Default),
		Protected:         boolValue(branch.Protected),
		Archived:          boolValue(branch.Archived),
		InitSource:        stringValue(branch.InitSource),
		CreationSource:    stringValue(branch.CreationSource),
		CreateTime:        stringValue(branch.CreateTime),
		UpdateTime:        stringValue(branch.UpdateTime),
		LastResetTime:     stringValue(branch.LastResetTime),
		StartParentLSN:    stringValue(branch.StartParentLSN),
		StartParentTime:   stringValue(branch.StartParentTime),
		StatusChangedTime: stringValue(branch.StatusChangedTime),
		ParentBranch:      mapUpdatedParentBranch(branch.ParentBranch),
		BranchUsage:       mapUpdatedBranchUsage(branch.BranchUsage),
	}
}

func mapUpdatedParentBranch(branch *aidap.ParentBranchForUpdateBranchOutput) ParentBranch {
	if branch == nil {
		return ParentBranch{}
	}
	return ParentBranch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
	}
}

func mapUpdatedBranchUsage(usage *aidap.BranchUsageForUpdateBranchOutput) BranchUsage {
	if usage == nil {
		return BranchUsage{}
	}
	return BranchUsage{
		WorkspaceID:          stringValue(usage.WorkspaceId),
		BranchID:             stringValue(usage.BranchId),
		ComputeTimeSeconds:   int64Value(usage.ComputeTimeSeconds),
		DataSizeTotalBytes:   int64Value(usage.DataSizeTotalBytes),
		DataSizeUsedBytes:    int64Value(usage.DataSizeUsedBytes),
		FunctionCallNum:      int64Value(usage.FunctionCallNum),
		LastRunningTime:      stringValue(usage.LastRunningTime),
		ServiceTimeSeconds:   int64Value(usage.ServiceTimeSeconds),
		StatTime:             stringValue(usage.StatTime),
		StorageSizeUsedBytes: int64Value(usage.StorageSizeUsedBytes),
	}
}

func mapSetAsDefaultBranchDetail(branch *aidap.BranchForSetAsDefaultBranchOutput) BranchDetail {
	if branch == nil {
		return BranchDetail{}
	}
	return BranchDetail{
		WorkspaceID:       stringValue(branch.WorkspaceId),
		BranchID:          stringValue(branch.BranchId),
		BranchName:        stringValue(branch.BranchName),
		BranchStatus:      stringValue(branch.BranchStatus),
		Default:           boolValue(branch.Default),
		Protected:         boolValue(branch.Protected),
		Archived:          boolValue(branch.Archived),
		InitSource:        stringValue(branch.InitSource),
		CreationSource:    stringValue(branch.CreationSource),
		CreateTime:        stringValue(branch.CreateTime),
		UpdateTime:        stringValue(branch.UpdateTime),
		LastResetTime:     stringValue(branch.LastResetTime),
		StartParentLSN:    stringValue(branch.StartParentLSN),
		StartParentTime:   stringValue(branch.StartParentTime),
		StatusChangedTime: stringValue(branch.StatusChangedTime),
		ParentBranch:      mapSetAsDefaultParentBranch(branch.ParentBranch),
		BranchUsage:       mapSetAsDefaultBranchUsage(branch.BranchUsage),
	}
}

func mapSetAsDefaultParentBranch(branch *aidap.ParentBranchForSetAsDefaultBranchOutput) ParentBranch {
	if branch == nil {
		return ParentBranch{}
	}
	return ParentBranch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
	}
}

func mapSetAsDefaultBranchUsage(usage *aidap.BranchUsageForSetAsDefaultBranchOutput) BranchUsage {
	if usage == nil {
		return BranchUsage{}
	}
	return BranchUsage{
		WorkspaceID:          stringValue(usage.WorkspaceId),
		BranchID:             stringValue(usage.BranchId),
		ComputeTimeSeconds:   int64Value(usage.ComputeTimeSeconds),
		DataSizeTotalBytes:   int64Value(usage.DataSizeTotalBytes),
		DataSizeUsedBytes:    int64Value(usage.DataSizeUsedBytes),
		FunctionCallNum:      int64Value(usage.FunctionCallNum),
		LastRunningTime:      stringValue(usage.LastRunningTime),
		ServiceTimeSeconds:   int64Value(usage.ServiceTimeSeconds),
		StatTime:             stringValue(usage.StatTime),
		StorageSizeUsedBytes: int64Value(usage.StorageSizeUsedBytes),
	}
}

func mapRestoreWindow(window *aidap.GetRestoreWindowOutput) RestoreWindow {
	if window == nil {
		return RestoreWindow{}
	}
	return RestoreWindow{
		WorkspaceID:        stringValue(window.WorkspaceId),
		BranchID:           stringValue(window.BranchId),
		WindowSizeSeconds:  int64Value(window.WindowSizeSeconds),
		BranchCreationTime: stringValue(window.BranchCreateTime),
		StartTime:          stringValue(window.StartTime),
		EndTime:            stringValue(window.EndTime),
	}
}

func mapDescribeRestorableBranchesResult(resp *aidap.DescribeRestorableBranchesOutput) DescribeRestorableBranchesResult {
	if resp == nil {
		return DescribeRestorableBranchesResult{}
	}
	result := DescribeRestorableBranchesResult{
		Total:         intValue(resp.Total),
		WorkspaceName: stringValue(resp.WorkspaceName),
	}
	for _, branch := range resp.Branches {
		result.Branches = append(result.Branches, mapRestorableBranch(branch))
	}
	for _, window := range resp.RestoreWindow {
		result.RestoreWindows = append(result.RestoreWindows, mapRestorableWindow(window))
	}
	return result
}

func mapRestorableBranch(branch *aidap.BranchForDescribeRestorableBranchesOutput) Branch {
	if branch == nil {
		return Branch{}
	}
	return Branch{
		WorkspaceID:  stringValue(branch.WorkspaceId),
		BranchID:     stringValue(branch.BranchId),
		BranchName:   stringValue(branch.BranchName),
		BranchStatus: stringValue(branch.BranchStatus),
		Default:      boolValue(branch.Default),
		Protected:    boolValue(branch.Protected),
		Archived:     boolValue(branch.Archived),
		InitSource:   stringValue(branch.InitSource),
		CreateTime:   stringValue(branch.CreateTime),
		UpdateTime:   stringValue(branch.UpdateTime),
	}
}

func mapRestorableWindow(window *aidap.RestoreWindowForDescribeRestorableBranchesOutput) RestoreWindow {
	if window == nil {
		return RestoreWindow{}
	}
	return RestoreWindow{
		BranchID:           stringValue(window.BranchId),
		WindowSizeSeconds:  int64Value(window.WindowSizeSeconds),
		BranchCreationTime: stringValue(window.BranchCreateTime),
		StartTime:          stringValue(window.StartTime),
		EndTime:            stringValue(window.EndTime),
	}
}

func mapDescribeAccessControlListResult(resp *aidap.DescribeAccessControlListOutput) DescribeAccessControlListResult {
	if resp == nil {
		return DescribeAccessControlListResult{}
	}
	result := DescribeAccessControlListResult{
		Total: intValue(resp.Total),
	}
	for _, data := range resp.Datas {
		result.AccessControlLists = append(result.AccessControlLists, mapAccessControlList(data))
	}
	return result
}

func mapAccessControlList(data *aidap.DataForDescribeAccessControlListOutput) AccessControlList {
	if data == nil {
		return AccessControlList{}
	}
	return AccessControlList{
		Name:    stringValue(data.AccessControlListName),
		AclType: stringValue(data.AclType),
		IPList:  stringPointersValue(data.IPList),
	}
}

func mapAPIKey(apiKey *aidap.APIKeyForDescribeAPIKeysOutput) APIKey {
	if apiKey == nil {
		return APIKey{}
	}
	return APIKey{
		Name:       stringValue(apiKey.Name),
		Key:        stringValue(apiKey.Key),
		Type:       stringValue(apiKey.Type),
		CreateTime: stringValue(apiKey.CreateTime),
	}
}

func mapDescribeComputesResult(resp *aidap.DescribeComputesOutput) DescribeComputesResult {
	if resp == nil {
		return DescribeComputesResult{}
	}
	result := DescribeComputesResult{
		Total: intValue(resp.Total),
	}
	for _, compute := range resp.Computes {
		result.Computes = append(result.Computes, mapCompute(compute))
	}
	return result
}

func mapCompute(compute *aidap.ComputeForDescribeComputesOutput) Compute {
	if compute == nil {
		return Compute{}
	}
	return Compute{
		WorkspaceID:           stringValue(compute.WorkspaceId),
		BranchID:              stringValue(compute.BranchId),
		ComputeID:             stringValue(compute.ComputeId),
		ComputeName:           stringValue(compute.ComputeName),
		ComputeStatus:         stringValue(compute.ComputeStatus),
		ComputeRole:           stringValue(compute.ComputeRole),
		ServiceType:           stringValue(compute.ServiceType),
		AutoScalingLimitMinCU: floatValue(compute.AutoScalingLimitMinCU),
		AutoScalingLimitMaxCU: floatValue(compute.AutoScalingLimitMaxCU),
		EnableAnalytics:       stringValue(compute.EnableAnalytic),
		CreationSource:        stringValue(compute.CreationSource),
		Disabled:              boolValue(compute.Disabled),
		CreateTime:            stringValue(compute.CreateTime),
		UpdateTime:            stringValue(compute.UpdateTime),
		LastActiveTime:        stringValue(compute.LastActiveTime),
		StatusChangedTime:     stringValue(compute.StatusChangedTime),
		SuspendedTime:         stringValue(compute.SuspendedTime),
	}
}

func mapComputeDetail(compute *aidap.ComputeForDescribeComputeDetailOutput) Compute {
	if compute == nil {
		return Compute{}
	}
	return Compute{
		WorkspaceID:           stringValue(compute.WorkspaceId),
		BranchID:              stringValue(compute.BranchId),
		ComputeID:             stringValue(compute.ComputeId),
		ComputeName:           stringValue(compute.ComputeName),
		ComputeStatus:         stringValue(compute.ComputeStatus),
		ComputeRole:           stringValue(compute.ComputeRole),
		ServiceType:           stringValue(compute.ServiceType),
		AutoScalingLimitMinCU: floatValue(compute.AutoScalingLimitMinCU),
		AutoScalingLimitMaxCU: floatValue(compute.AutoScalingLimitMaxCU),
		EnableAnalytics:       stringValue(compute.EnableAnalytic),
		CreationSource:        stringValue(compute.CreationSource),
		Disabled:              boolValue(compute.Disabled),
		CreateTime:            stringValue(compute.CreateTime),
		UpdateTime:            stringValue(compute.UpdateTime),
		LastActiveTime:        stringValue(compute.LastActiveTime),
		StatusChangedTime:     stringValue(compute.StatusChangedTime),
		SuspendedTime:         stringValue(compute.SuspendedTime),
	}
}

func mapModifiedCompute(compute *aidap.ComputeForModifyComputeSpecOutput) Compute {
	if compute == nil {
		return Compute{}
	}
	return Compute{
		WorkspaceID:           stringValue(compute.WorkspaceId),
		BranchID:              stringValue(compute.BranchId),
		ComputeID:             stringValue(compute.ComputeId),
		ComputeName:           stringValue(compute.ComputeName),
		ComputeStatus:         stringValue(compute.ComputeStatus),
		ComputeRole:           stringValue(compute.ComputeRole),
		ServiceType:           stringValue(compute.ServiceType),
		AutoScalingLimitMinCU: floatValue(compute.AutoScalingLimitMinCU),
		AutoScalingLimitMaxCU: floatValue(compute.AutoScalingLimitMaxCU),
		EnableAnalytics:       stringValue(compute.EnableAnalytic),
		CreationSource:        stringValue(compute.CreationSource),
		Disabled:              boolValue(compute.Disabled),
		CreateTime:            stringValue(compute.CreateTime),
		UpdateTime:            stringValue(compute.UpdateTime),
		LastActiveTime:        stringValue(compute.LastActiveTime),
		StatusChangedTime:     stringValue(compute.StatusChangedTime),
		SuspendedTime:         stringValue(compute.SuspendedTime),
	}
}
