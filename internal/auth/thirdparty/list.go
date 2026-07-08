// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package thirdparty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type ThirdPartyProvider struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	OIDCIssuerURL string `json:"oidc_issuer_url,omitempty"`
	ResolvedAt    string `json:"resolved_at,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

func RunVolcengineList(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) error {
	providers, err := ListThirdPartyProviders(ctx, client, workspaceID, branchID)
	if err != nil {
		return err
	}
	return outputProviders(providers)
}

func ListThirdPartyProviders(ctx context.Context, client *volcengine.Client, workspaceID, branchID string) ([]ThirdPartyProvider, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	body, err := access.DoRequest(ctx, http.MethodGet, "/auth/v1/config/third-party-auth", nil)
	if err != nil {
		return nil, err
	}
	var providers []ThirdPartyProvider
	if err := json.Unmarshal(body, &providers); err != nil {
		return nil, errors.Errorf("failed to decode third-party list response: %w", err)
	}
	return providers, nil
}

func outputProviders(providers []ThirdPartyProvider) error {
	if len(providers) == 0 {
		fmt.Println("No third-party auth providers configured.")
		return nil
	}
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := "|ID|TYPE|OIDC ISSUER URL|RESOLVED AT|\n|-|-|-|-|\n"
		for _, p := range providers {
			table += fmt.Sprintf("|`%s`|`%s`|`%s`|`%s`|\n", p.ID, p.Type, p.OIDCIssuerURL, p.ResolvedAt)
		}
		return utils.RenderTable(table)
	default:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, providers)
	}
}
