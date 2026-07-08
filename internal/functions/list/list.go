// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package list

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

func Run(ctx context.Context, projectRef string, fsys afero.Fs) error {
	resp, err := utils.GetSupabase().V1ListAllFunctionsWithResponse(ctx, projectRef)
	if err != nil {
		return errors.Errorf("failed to list functions: %w", err)
	} else if resp.JSON200 == nil {
		return errors.Errorf("unexpected list functions status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return outputFunctions(*resp.JSON200)
}

func outputFunctions(functions []api.FunctionResponse) error {
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		var table strings.Builder
		table.WriteString(`|ID|NAME|SLUG|STATUS|VERSION|UPDATED_AT (UTC)|
|-|-|-|-|-|-|
`)
		for _, function := range functions {
			t := time.UnixMilli(function.UpdatedAt)
			fmt.Fprintf(&table, "|`%s`|`%s`|`%s`|`%s`|`%d`|`%s`|\n",
				function.Id,
				function.Name,
				function.Slug,
				function.Status,
				function.Version,
				t.UTC().Format("2006-01-02 15:04:05"),
			)
		}
		return utils.RenderTable(table.String())
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Functions []api.FunctionResponse `toml:"functions"`
		}{
			Functions: functions,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}

	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, functions)
}
