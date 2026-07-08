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
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

func Run(ctx context.Context, projectRef string, fsys afero.Fs) error {
	secrets, err := GetSecretDigests(ctx, projectRef)
	if err != nil {
		return err
	}
	return outputSecrets(secrets)
}

func outputSecrets(secrets []api.SecretResponse) error {
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := `|NAME|DIGEST|
|-|-|
`
		for _, secret := range secrets {
			table += fmt.Sprintf("|`%s`|`%s`|\n", strings.ReplaceAll(secret.Name, "|", "\\|"), secret.Value)
		}
		return utils.RenderTable(table)
	case utils.OutputToml:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, struct {
			Secrets []api.SecretResponse `toml:"secrets"`
		}{
			Secrets: secrets,
		})
	case utils.OutputEnv:
		return errors.New(utils.ErrEnvNotSupported)
	}

	return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, secrets)
}

func GetSecretDigests(ctx context.Context, projectRef string) ([]api.SecretResponse, error) {
	resp, err := utils.GetSupabase().V1ListAllSecretsWithResponse(ctx, projectRef)
	if err != nil {
		return nil, errors.Errorf("failed to list secrets: %w", err)
	} else if resp.JSON200 == nil {
		return nil, errors.Errorf("unexpected list secrets status %d: %s", resp.StatusCode(), string(resp.Body))
	}
	secrets := *resp.JSON200
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].Name < secrets[j].Name
	})
	return secrets, nil
}
