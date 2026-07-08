// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package network

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func ListVPCs(ctx context.Context, limit int) error {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	vpcs, err := volcengine.NewClient(cfg).DescribeVPCs(ctx, limit)
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(vpcsMarkdown(vpcs))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			VPCs []volcengine.VPCNetwork `toml:"vpcs"`
		}{VPCs: vpcs})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, vpcs)
}

func ListSubnets(ctx context.Context, vpcID string, limit int) error {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	subnets, err := volcengine.NewClient(cfg).DescribeSubnets(ctx, vpcID, limit)
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(subnetsMarkdown(subnets))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Subnets []volcengine.Subnet `toml:"subnets"`
		}{Subnets: subnets})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, subnets)
}

func vpcsMarkdown(vpcs []volcengine.VPCNetwork) string {
	var table strings.Builder
	table.WriteString(`|VPC ID|NAME|CIDR|IPV6 CIDR|STATUS|PROJECT|DEFAULT|
|-|-|-|-|-|-|-|
`)
	for _, vpc := range vpcs {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%t`|\n",
			vpc.VPCID,
			escape(vpc.VPCName),
			vpc.CIDRBlock,
			vpc.IPv6CIDRBlock,
			vpc.Status,
			escape(vpc.ProjectName),
			vpc.Default)
	}
	return table.String()
}

func subnetsMarkdown(subnets []volcengine.Subnet) string {
	var table strings.Builder
	table.WriteString(`|SUBNET ID|NAME|VPC ID|CIDR|IPV6 CIDR|STATUS|ZONE|AVAILABLE IPV4|DEFAULT|
|-|-|-|-|-|-|-|-|-|
`)
	for _, subnet := range subnets {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%s`|`%d`|`%t`|\n",
			subnet.SubnetID,
			escape(subnet.SubnetName),
			subnet.VPCID,
			subnet.CIDRBlock,
			subnet.IPv6CIDRBlock,
			subnet.Status,
			subnet.ZoneID,
			subnet.AvailableIPAddressCount,
			subnet.Default)
	}
	return table.String()
}

func escape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
