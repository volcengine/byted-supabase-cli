// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package thirdparty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type AddProviderParams struct {
	OIDCIssuerURL       string
	CustomJWKSKid       string
	CustomJWKSAlgorithm string
	CustomJWKSPublicKey string
}

// addProviderRequest is the request body structure sent to the Auth API
type addProviderRequest struct {
	OIDCIssuerURL   string              `json:"oidc_issuer_url,omitempty"`
	CustomJWTSecret *customJWTSecretReq `json:"custom_jwt_secret,omitempty"`
}

type customJWTSecretReq struct {
	KID       string `json:"kid"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"public_key"`
}

// AddProvider creates a Third-Party Auth provider and returns the created provider
// object decoded from the Auth Admin API response. Data-layer helper shared by the CLI
// (RunVolcengineAdd) and the MCP server.
func AddProvider(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, params AddProviderParams) (map[string]interface{}, error) {
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return nil, err
	}
	// Build Auth API request body
	reqBody := addProviderRequest{}
	if params.OIDCIssuerURL != "" {
		reqBody.OIDCIssuerURL = params.OIDCIssuerURL
	} else if params.CustomJWKSKid != "" || params.CustomJWKSAlgorithm != "" || params.CustomJWKSPublicKey != "" {
		// Convert literal \n to real newlines for convenience when passing single-line PEM in shell
		publicKey := strings.ReplaceAll(params.CustomJWKSPublicKey, `\n`, "\n")
		reqBody.CustomJWTSecret = &customJWTSecretReq{
			KID:       params.CustomJWKSKid,
			Algorithm: params.CustomJWKSAlgorithm,
			PublicKey: publicKey,
		}
	} else {
		return nil, errors.New("either --oidc-issuer-url or --jwks-kid/--jwks-algorithm/--jwks-public-key is required")
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Errorf("failed to marshal add provider request: %w", err)
	}
	respBody, err := access.DoRequest(ctx, http.MethodPost, "/auth/v1/config/third-party-auth", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, errors.Errorf("failed to decode add provider response: %w", err)
	}
	return result, nil
}

func RunVolcengineAdd(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, params AddProviderParams) error {
	result, err := AddProvider(ctx, client, workspaceID, branchID, params)
	if err != nil {
		return err
	}
	// Output creation result
	switch utils.OutputFormat.Value {
	case utils.OutputPretty:
		table := "|KEY|VALUE|\n|-|-|\n"
		for _, k := range []string{"id", "type", "oidc_issuer_url", "resolved_at", "created_at"} {
			if v, ok := result[k]; ok && v != nil {
				table += fmt.Sprintf("|`%s`|`%s`|\n", k, fmt.Sprint(v))
			}
		}
		return utils.RenderTable(table)
	default:
		return utils.EncodeOutput(utils.OutputFormat.Value, os.Stdout, result)
	}
}
