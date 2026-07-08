// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
)

// PgMetaAccess contains the temporary branch-scoped values needed to call pg-meta.
type PgMetaAccess struct {
	BranchID       string
	BaseURL        string
	ServiceRoleKey string
}

// ResolveDefaultBranchID returns branchID when non-empty, otherwise resolves the
// workspace's default branch. Single source of truth for the "default branch
// when none is given" fallback shared across the CLI and MCP tools.
func (c *Client) ResolveDefaultBranchID(ctx context.Context, workspaceID, branchID string) (string, error) {
	branchID = strings.TrimSpace(branchID)
	if branchID != "" {
		return branchID, nil
	}
	defaultBranch, err := c.DescribeDefaultBranch(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	branchID = strings.TrimSpace(defaultBranch.Branch.BranchID)
	if branchID == "" {
		return "", errors.Errorf("failed to resolve default branch for workspace %s", workspaceID)
	}
	return branchID, nil
}

// ResolveBranchBaseURL resolves the branch's pg-meta base URL. When branchID is
// empty it falls back to the workspace's default branch. It is the single source
// of truth for the "resolve branch → describe endpoints → pick base URL" path,
// shared by ResolvePgMetaAccess and callers that only need the URL (e.g. the MCP
// get_workspace_url tool).
func (c *Client) ResolveBranchBaseURL(ctx context.Context, workspaceID, branchID string) (baseURL, resolvedBranchID string, err error) {
	branchID, err = c.ResolveDefaultBranchID(ctx, workspaceID, branchID)
	if err != nil {
		return "", "", err
	}
	endpoints, err := c.DescribeWorkspaceEndpoints(ctx, DescribeWorkspaceEndpointsParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
	})
	if err != nil {
		return "", "", err
	}
	baseURL, err = ResolvePgMetaBaseURL(endpoints.Endpoints)
	if err != nil {
		return "", "", err
	}
	return baseURL, branchID, nil
}

// ResolvePgMetaAccess resolves the branch endpoint and service role key in memory.
func (c *Client) ResolvePgMetaAccess(ctx context.Context, workspaceID, branchID string) (PgMetaAccess, error) {
	baseURL, branchID, err := c.ResolveBranchBaseURL(ctx, workspaceID, branchID)
	if err != nil {
		return PgMetaAccess{}, err
	}
	keys, err := c.DescribeAPIKeys(ctx, DescribeAPIKeysParams{
		WorkspaceID: workspaceID,
		BranchID:    branchID,
		Limit:       100,
	})
	if err != nil {
		return PgMetaAccess{}, err
	}
	serviceRoleKey := FindServiceRoleKey(keys.APIKeys)
	if serviceRoleKey == "" {
		return PgMetaAccess{}, errors.New("service role API key not found for selected branch")
	}
	return PgMetaAccess{BranchID: branchID, BaseURL: baseURL, ServiceRoleKey: serviceRoleKey}, nil
}

func ResolvePgMetaBaseURL(endpoints []Endpoint) (string, error) {
	for _, endpoint := range endpoints {
		for _, address := range endpoint.Addresses {
			if strings.EqualFold(address.AddressType, "Public") {
				return endpointBaseURL(address)
			}
		}
	}
	for _, endpoint := range endpoints {
		if !strings.EqualFold(endpoint.EndpointType, "DashBoard") {
			continue
		}
		for _, preferredType := range []string{"Inner", "Private"} {
			for _, address := range endpoint.Addresses {
				if strings.EqualFold(address.AddressType, preferredType) {
					return endpointBaseURL(address)
				}
			}
		}
	}
	return "", errors.New("no usable Supabase endpoint address found for pg-meta access")
}

func BuildPgMetaURL(baseURL, route string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Hostname() == "" {
		return "", errors.Errorf("invalid pg-meta base URL %q", baseURL)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(route, "/")
	return parsed.String(), nil
}

// RoleKeys is a branch's API keys classified by role.
type RoleKeys struct {
	Publishable string
	Anon        string
	ServiceRole string
	All         []APIKey
}

// ExtractRoleKeys classifies a branch's API keys by role: the public key serves
// as both publishable and anon; the service-role key is matched by type
// "Service" or name "service_role". Single source of truth for key-role
// extraction, shared by the CLI and the MCP tools.
func ExtractRoleKeys(keys []APIKey) RoleKeys {
	out := RoleKeys{All: keys}
	for _, key := range keys {
		switch {
		case strings.EqualFold(key.Type, "Public"):
			out.Publishable = key.Key
			out.Anon = key.Key
		case strings.EqualFold(key.Type, "Service") || strings.EqualFold(key.Name, "service_role"):
			out.ServiceRole = key.Key
		}
	}
	return out
}

func FindServiceRoleKey(keys []APIKey) string {
	return ExtractRoleKeys(keys).ServiceRole
}

func endpointBaseURL(address EndpointAddress) (string, error) {
	domain := strings.TrimSpace(address.AddressDomain)
	if domain == "" {
		return "", errors.New("empty endpoint domain")
	}
	scheme := "https"
	if address.AddressPort == 80 {
		scheme = "http"
	}
	if !strings.Contains(domain, "://") {
		domain = scheme + "://" + domain
	}
	parsed, err := url.Parse(domain)
	if err != nil || parsed.Hostname() == "" {
		return "", errors.Errorf("invalid endpoint domain %q", address.AddressDomain)
	}
	if address.AddressPort > 0 {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(address.AddressPort))
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}
