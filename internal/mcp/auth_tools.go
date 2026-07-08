// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"strings"

	"github.com/go-errors/errors"
	authconfig "github.com/volcengine/byted-supabase-cli/internal/auth/config"
	authhooks "github.com/volcengine/byted-supabase-cli/internal/auth/hooks"
	authtp "github.com/volcengine/byted-supabase-cli/internal/auth/thirdparty"
)

// authTools registers the auth group: Supabase Auth general config, Auth Hooks, and
// Third-Party Auth providers for a branch.
//
// Thin adapters over the fork-specific data-layer helpers in internal/auth/{config,hooks,
// thirdparty} (GetAuthConfig / SetAuthConfig / GetHooksConfig / SetHooksConfig /
// ListThirdPartyProviders / AddProvider / RemoveProvider / SyncProviders), which go through
// the pg-meta Auth Admin API; they never call the cobra command Run() functions. An empty
// branch_id resolves to the workspace's default branch inside ResolvePgMetaAccess.
func authTools() []toolSpec {
	return []toolSpec{
		defineTool(meta{
			name:        "get_auth_config",
			title:       "Get Auth config",
			feature:     featureAuth,
			description: "Get the Supabase Auth general configuration (signup, email/SMS, JWT, sessions, rate limits, SMTP, ...) for a branch as a JSON object of config keys.",
		}, getAuthConfig),
		defineTool(meta{
			name:        "update_auth_config",
			title:       "Update Auth config",
			feature:     featureAuth,
			mutating:    true,
			description: "Update the Supabase Auth general configuration for a branch. Pass config as a JSON object of the keys to change (e.g. {\"DISABLE_SIGNUP\": true, \"SITE_URL\": \"https://example.com\"}); only the supplied keys are modified.",
		}, updateAuthConfig),
		defineTool(meta{
			name:        "get_auth_hooks_config",
			title:       "Get Auth Hooks config",
			feature:     featureAuth,
			description: "Get the Supabase Auth Hooks configuration (custom access token, send SMS/email, MFA verification, password verification hooks) for a branch as a JSON object.",
		}, getAuthHooksConfig),
		defineTool(meta{
			name:        "update_auth_hooks_config",
			title:       "Update Auth Hooks config",
			feature:     featureAuth,
			mutating:    true,
			description: "Update the Supabase Auth Hooks configuration for a branch. Pass config as a JSON object of the keys to change; only the supplied keys are modified.",
		}, updateAuthHooksConfig),
		defineTool(meta{
			name:        "list_third_party_auth",
			title:       "List third-party auth providers",
			feature:     featureAuth,
			description: "List the Third-Party Auth providers (external JWT issuers) configured for a branch.",
		}, listThirdPartyAuth),
		defineTool(meta{
			name:        "create_third_party_auth",
			title:       "Create third-party auth provider",
			feature:     featureAuth,
			mutating:    true,
			description: "Add a Third-Party Auth provider to a branch. Provide oidc_issuer_url for a standard OIDC provider, or jwks_kid + jwks_algorithm + jwks_public_key for a custom JWKS provider (the two modes are mutually exclusive).",
		}, createThirdPartyAuth),
		defineTool(meta{
			name:        "delete_third_party_auth",
			title:       "Delete third-party auth provider",
			feature:     featureAuth,
			mutating:    true,
			description: "Remove a Third-Party Auth provider by id. Requests authenticated with that provider's JWT will immediately start failing.",
		}, deleteThirdPartyAuth),
		defineTool(meta{
			name:        "sync_third_party_auth",
			title:       "Sync third-party auth JWKS",
			feature:     featureAuth,
			mutating:    true,
			description: "Trigger an immediate JWKS public key sync for all Third-Party Auth providers of a branch. Use after rotating a provider's JWKS keys.",
		}, syncThirdPartyAuth),
	}
}

// authTarget is the workspace/branch selector shared by the read-only and action auth tools.
type authTarget struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func getAuthConfig(ctx context.Context, p *policy, in authTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	cfg, err := authconfig.GetAuthConfig(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(cfg)
}

type updateAuthConfigInput struct {
	Config      map[string]any `json:"config" jsonschema:"JSON object of Auth config key/value pairs to set; only the provided keys are changed"`
	WorkspaceID string         `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string         `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func updateAuthConfig(ctx context.Context, p *policy, in updateAuthConfigInput) (string, error) {
	if len(in.Config) == 0 {
		return "", errors.New("config is required: pass a JSON object of the keys to set")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := authconfig.SetAuthConfig(ctx, client, workspaceID, strings.TrimSpace(in.BranchID), in.Config)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"config":  result,
	})
}

func getAuthHooksConfig(ctx context.Context, p *policy, in authTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	cfg, err := authhooks.GetHooksConfig(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(cfg)
}

type updateAuthHooksConfigInput struct {
	Config      map[string]any `json:"config" jsonschema:"JSON object of Auth Hooks config key/value pairs to set; only the provided keys are changed"`
	WorkspaceID string         `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string         `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func updateAuthHooksConfig(ctx context.Context, p *policy, in updateAuthHooksConfigInput) (string, error) {
	if len(in.Config) == 0 {
		return "", errors.New("config is required: pass a JSON object of the keys to set")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	result, err := authhooks.SetHooksConfig(ctx, client, workspaceID, strings.TrimSpace(in.BranchID), in.Config)
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"config":  result,
	})
}

func listThirdPartyAuth(ctx context.Context, p *policy, in authTarget) (string, error) {
	client, workspaceID, err := p.client(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	providers, err := authtp.ListThirdPartyProviders(ctx, client, workspaceID, strings.TrimSpace(in.BranchID))
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"providers": providers,
		"count":     len(providers),
	})
}

type createThirdPartyAuthInput struct {
	OIDCIssuerURL string `json:"oidc_issuer_url,omitempty" jsonschema:"OIDC issuer URL for a standard OIDC provider; mutually exclusive with the jwks_* fields"`
	JWKSKid       string `json:"jwks_kid,omitempty" jsonschema:"custom JWKS key id; use with jwks_algorithm and jwks_public_key instead of oidc_issuer_url"`
	JWKSAlgorithm string `json:"jwks_algorithm,omitempty" jsonschema:"custom JWKS signing algorithm, e.g. RS256 or ES256"`
	JWKSPublicKey string `json:"jwks_public_key,omitempty" jsonschema:"custom JWKS public key in PEM format"`
	WorkspaceID   string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID      string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func createThirdPartyAuth(ctx context.Context, p *policy, in createThirdPartyAuthInput) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	// AddProvider validates that exactly one of OIDC / custom-JWKS is provided.
	provider, err := authtp.AddProvider(ctx, client, workspaceID, strings.TrimSpace(in.BranchID), authtp.AddProviderParams{
		OIDCIssuerURL:       strings.TrimSpace(in.OIDCIssuerURL),
		CustomJWKSKid:       strings.TrimSpace(in.JWKSKid),
		CustomJWKSAlgorithm: strings.TrimSpace(in.JWKSAlgorithm),
		CustomJWKSPublicKey: in.JWKSPublicKey,
	})
	if err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":  true,
		"provider": provider,
	})
}

type deleteThirdPartyAuthInput struct {
	ProviderID  string `json:"provider_id" jsonschema:"id of the Third-Party Auth provider to remove"`
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"Supabase workspace (project) id; omit when the server is scoped to a single workspace"`
	BranchID    string `json:"branch_id,omitempty" jsonschema:"branch id; defaults to the workspace's default branch"`
}

func deleteThirdPartyAuth(ctx context.Context, p *policy, in deleteThirdPartyAuthInput) (string, error) {
	providerID := strings.TrimSpace(in.ProviderID)
	if providerID == "" {
		return "", errors.New("provider_id is required")
	}
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if err := authtp.RemoveProvider(ctx, client, workspaceID, strings.TrimSpace(in.BranchID), providerID); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success":     true,
		"message":     "Third-party auth provider removed successfully",
		"provider_id": providerID,
	})
}

func syncThirdPartyAuth(ctx context.Context, p *policy, in authTarget) (string, error) {
	client, workspaceID, err := p.writeClientFor(ctx, in.WorkspaceID)
	if err != nil {
		return "", err
	}
	if err := authtp.SyncProviders(ctx, client, workspaceID, strings.TrimSpace(in.BranchID)); err != nil {
		return "", err
	}
	return toJSON(map[string]any{
		"success": true,
		"message": "Third-party auth provider JWKS keys synced successfully",
	})
}
