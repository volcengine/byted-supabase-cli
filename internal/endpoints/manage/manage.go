// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package manage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

const eipStatusAvailable = "Available"

const (
	internetProtocolIPv4      = "IPv4"
	internetProtocolDualStack = "DualStack"
)

type EnablePublicParams struct {
	WorkspaceID string
	BranchID    string
	EndpointID  string
	EIPID       string
}

type EnablePublicResult struct {
	WorkspaceID string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
	BranchID    string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	Mode        string `json:"Mode" toml:"mode" yaml:"mode"`
	EndpointID  string `json:"EndpointId,omitempty" toml:"endpoint_id,omitempty" yaml:"endpoint_id,omitempty"`
	EIPID       string `json:"EipId,omitempty" toml:"eip_id,omitempty" yaml:"eip_id,omitempty"`
}

type DisablePublicParams struct {
	WorkspaceID string
	BranchID    string
	EndpointID  string
}

type DisablePublicResult struct {
	WorkspaceID string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
	BranchID    string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	Mode        string `json:"Mode" toml:"mode" yaml:"mode"`
	EndpointID  string `json:"EndpointId,omitempty" toml:"endpoint_id,omitempty" yaml:"endpoint_id,omitempty"`
}

type EnablePrivateParams struct {
	WorkspaceID      string
	BranchID         string
	VPCID            string
	SubnetID         string
	InternetProtocol string
}

type EnablePrivateResult struct {
	WorkspaceID      string `json:"WorkspaceId" toml:"workspace_id" yaml:"workspace_id"`
	BranchID         string `json:"BranchId" toml:"branch_id" yaml:"branch_id"`
	VPCID            string `json:"VpcId" toml:"vpc_id" yaml:"vpc_id"`
	SubnetID         string `json:"SubnetId" toml:"subnet_id" yaml:"subnet_id"`
	InternetProtocol string `json:"InternetProtocol" toml:"internet_protocol" yaml:"internet_protocol"`
}

func EnablePublic(ctx context.Context, params EnablePublicParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, params.WorkspaceID)
	if err != nil {
		return err
	}
	client := volcengine.NewClient(cfg)
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	endpointID := strings.TrimSpace(params.EndpointID)
	eipID := strings.TrimSpace(params.EIPID)
	if eipID != "" && endpointID == "" {
		return errors.New("--eip-id requires --endpoint-id for dedicated public endpoint access")
	}
	mode := "shared"
	if endpointID != "" {
		mode = "dedicated"
		if eipID == "" {
			eipID, err = promptAvailableEIPAddress(ctx, client)
			if err != nil {
				return err
			}
		}
		eip, err := client.DescribeEIPAddress(ctx, eipID)
		if err != nil {
			return err
		}
		if eip.Status != eipStatusAvailable || eip.InstanceID != "" {
			return errors.Errorf("EIP %s is not available for binding", eipID)
		}
	}
	action := "enable shared public endpoint access"
	if mode == "dedicated" {
		action = "enable dedicated public endpoint access"
	}
	title := fmt.Sprintf("Do you want to %s for branch %s?", action, utils.Aqua(branchID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	if err := volcengine.NewWriteClient(cfg).CreateEndpointPublicAddress(ctx, volcengine.CreateEndpointPublicAddressParams{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
		EndpointID:  endpointID,
		EIPID:       eipID,
	}); err != nil {
		return err
	}
	result := EnablePublicResult{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
		Mode:        mode,
		EndpointID:  endpointID,
		EIPID:       eipID,
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		fmt.Printf("Enabled %s public endpoint access for branch %s.\n", mode, utils.Aqua(branchID))
		return nil
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func DisablePublic(ctx context.Context, params DisablePublicParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, params.WorkspaceID)
	if err != nil {
		return err
	}
	client := volcengine.NewClient(cfg)
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	endpointID := strings.TrimSpace(params.EndpointID)
	mode := "shared"
	if endpointID != "" {
		mode = "dedicated"
	}
	title := fmt.Sprintf("Do you want to disable %s public endpoint access for branch %s?", mode, utils.Aqua(branchID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	if err := volcengine.NewWriteClient(cfg).DeleteEndpointPublicAddress(ctx, volcengine.DeleteEndpointPublicAddressParams{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
		EndpointID:  endpointID,
	}); err != nil {
		return err
	}
	result := DisablePublicResult{
		WorkspaceID: workspace.WorkspaceID,
		BranchID:    branchID,
		Mode:        mode,
		EndpointID:  endpointID,
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		fmt.Printf("Disabled %s public endpoint access for branch %s.\n", mode, utils.Aqua(branchID))
		return nil
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func EnablePrivate(ctx context.Context, params EnablePrivateParams) error {
	cfgSet, err := volcengine.LoadConfigSetFromEnv()
	if err != nil {
		return err
	}
	workspace, cfg, err := findSupabaseWorkspace(ctx, cfgSet, params.WorkspaceID)
	if err != nil {
		return err
	}
	client := volcengine.NewClient(cfg)
	branchID, err := resolveBranchID(ctx, client, workspace.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	vpcID := strings.TrimSpace(params.VPCID)
	if vpcID == "" {
		vpcID, err = promptVPC(ctx, client)
		if err != nil {
			return err
		}
	}
	subnetID := strings.TrimSpace(params.SubnetID)
	if subnetID == "" {
		subnetID, err = promptSubnet(ctx, client, vpcID)
		if err != nil {
			return err
		}
	}
	protocol := strings.TrimSpace(params.InternetProtocol)
	if protocol == "" {
		protocol = internetProtocolIPv4
	}
	if protocol != internetProtocolIPv4 && protocol != internetProtocolDualStack {
		return errors.New("--internet-protocol must be IPv4 or DualStack")
	}
	title := fmt.Sprintf("Do you want to enable private endpoint access for branch %s using VPC %s and subnet %s?",
		utils.Aqua(branchID), utils.Aqua(vpcID), utils.Aqua(subnetID))
	if shouldRun, err := utils.NewConsole().PromptYesNo(ctx, title, false); err != nil {
		return err
	} else if !shouldRun {
		return errors.New(context.Canceled)
	}
	if err := volcengine.NewWriteClient(cfg).ModifyVpcSettings(ctx, volcengine.ModifyVpcSettingsParams{
		WorkspaceID:      workspace.WorkspaceID,
		BranchID:         branchID,
		VPCID:            vpcID,
		SubnetID:         subnetID,
		InternetProtocol: protocol,
	}); err != nil {
		return err
	}
	result := EnablePrivateResult{
		WorkspaceID:      workspace.WorkspaceID,
		BranchID:         branchID,
		VPCID:            vpcID,
		SubnetID:         subnetID,
		InternetProtocol: protocol,
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		fmt.Printf("Enabled private endpoint access for branch %s using VPC %s and subnet %s.\n",
			utils.Aqua(branchID), utils.Aqua(vpcID), utils.Aqua(subnetID))
		return nil
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
}

func resolveBranchID(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) (string, error) {
	branchID = strings.TrimSpace(branchID)
	if branchID != "" {
		return branchID, nil
	}
	result, err := client.DescribeDefaultBranch(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if result.Branch.BranchID == "" {
		return "", errors.Errorf("failed to resolve default branch for workspace %s", workspaceID)
	}
	return result.Branch.BranchID, nil
}

func promptAvailableEIPAddress(ctx context.Context, client *volcengine.Client) (string, error) {
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing --eip-id. Supply both --endpoint-id and --eip-id for dedicated public endpoint access.")
	}
	eips, err := client.DescribeAvailableEIPAddresses(ctx)
	if err != nil {
		return "", err
	}
	if len(eips) == 0 {
		return "", errors.New("no available EIP found for dedicated public endpoint access")
	}
	items := make([]utils.PromptItem, len(eips))
	for i, eip := range eips {
		details := fmt.Sprintf("address: %s, bandwidth: %d Mbps, isp: %s", eip.Address, eip.Bandwidth, eip.ISP)
		if eip.Name != "" {
			details = "name: " + eip.Name + ", " + details
		}
		items[i] = utils.PromptItem{Summary: eip.AllocationID, Details: details}
	}
	choice, err := utils.PromptChoice(ctx, "Select an available EIP:", items)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "Selected EIP:", choice.Summary)
	return choice.Summary, nil
}

func promptVPC(ctx context.Context, client *volcengine.Client) (string, error) {
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing required flag: --vpc-id")
	}
	vpcs, err := client.DescribeVPCs(ctx, 100)
	if err != nil {
		return "", err
	}
	if len(vpcs) == 0 {
		return "", errors.New("no VPC found in the selected region")
	}
	items := make([]utils.PromptItem, len(vpcs))
	for i, vpc := range vpcs {
		items[i] = utils.PromptItem{
			Summary: vpc.VPCID,
			Details: fmt.Sprintf("name: %s, cidr: %s, status: %s", vpc.VPCName, vpc.CIDRBlock, vpc.Status),
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a VPC for private endpoint access:", items)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "Selected VPC:", choice.Summary)
	return choice.Summary, nil
}

func promptSubnet(ctx context.Context, client *volcengine.Client, vpcID string) (string, error) {
	console := utils.NewConsole()
	if !console.IsTTY {
		return "", errors.New("missing required flag: --subnet-id")
	}
	subnets, err := client.DescribeSubnets(ctx, vpcID, 100)
	if err != nil {
		return "", err
	}
	if len(subnets) == 0 {
		return "", errors.Errorf("no subnet found in VPC %s", vpcID)
	}
	items := make([]utils.PromptItem, len(subnets))
	for i, subnet := range subnets {
		items[i] = utils.PromptItem{
			Summary: subnet.SubnetID,
			Details: fmt.Sprintf("name: %s, cidr: %s, zone: %s, status: %s", subnet.SubnetName, subnet.CIDRBlock, subnet.ZoneID, subnet.Status),
		}
	}
	choice, err := utils.PromptChoice(ctx, "Select a subnet for private endpoint access:", items)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "Selected subnet:", choice.Summary)
	return choice.Summary, nil
}

func findSupabaseWorkspace(ctx context.Context, cfgSet volcengine.ConfigSet, workspaceID string) (volcengine.Workspace, volcengine.Config, error) {
	var lastErr error
	for _, region := range cfgSet.Regions {
		cfg := cfgSet.ConfigForRegion(region)
		result, err := volcengine.NewClient(cfg).DescribeWorkspaceDetail(ctx, workspaceID)
		if err != nil {
			lastErr = err
			continue
		}
		workspace := result.Workspace
		if workspace.WorkspaceID == "" {
			continue
		}
		if workspace.EngineType != "" && workspace.EngineType != volcengine.EngineTypeSupabase {
			return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("workspace %s is %s, expected Supabase", workspaceID, workspace.EngineType)
		}
		return workspace, cfg, nil
	}
	if lastErr != nil {
		return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("failed to describe volcengine project %s: %w", workspaceID, lastErr)
	}
	return volcengine.Workspace{}, volcengine.Config{}, errors.Errorf("volcengine project %s not found", workspaceID)
}
