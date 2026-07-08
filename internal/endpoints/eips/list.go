// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package eips

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func List(ctx context.Context, includeAll bool, limit int) error {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	eips, err := volcengine.NewClient(cfg).DescribeEIPAddresses(ctx, volcengine.DescribeEIPAddressesParams{
		AvailableOnly: !includeAll,
		Limit:         limit,
	})
	if err != nil {
		return err
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		return utils.RenderTable(toMarkdown(eips))
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			EIPAddresses []volcengine.EIPAddress `toml:"eip_addresses"`
		}{
			EIPAddresses: eips,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}
	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, eips)
}

func toMarkdown(eips []volcengine.EIPAddress) string {
	var table strings.Builder
	table.WriteString(`|EIP ID|ADDRESS|NAME|STATUS|BANDWIDTH (MBPS)|ISP|BOUND INSTANCE|
|-|-|-|-|-|-|-|
`)
	for _, eip := range eips {
		fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%d`|`%s`|`%s`|\n",
			eip.AllocationID,
			eip.Address,
			escape(eip.Name),
			eip.Status,
			eip.Bandwidth,
			eip.ISP,
			escape(eip.InstanceID))
	}
	return table.String()
}

func escape(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
