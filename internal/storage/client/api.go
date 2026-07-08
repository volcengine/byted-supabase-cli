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

package client

import (
	"context"
	"net/http"

	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/internal/status"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/tenant"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/fetcher"
	"github.com/volcengine/byted-supabase-cli/pkg/storage"
)

func NewStorageAPI(ctx context.Context, projectRef string) (storage.StorageAPI, error) {
	client := storage.StorageAPI{}
	if len(projectRef) == 0 {
		client.Fetcher = newLocalClient()
	} else if viper.IsSet("AUTH_SERVICE_ROLE_KEY") {
		// Special case for calling storage API without personal access token
		client.Fetcher = newRemoteClient(projectRef, utils.Config.Auth.ServiceRoleKey.Value)
	} else if apiKey, err := tenant.GetApiKeys(ctx, projectRef); err == nil {
		client.Fetcher = newRemoteClient(projectRef, apiKey.ServiceRole)
	} else {
		return client, err
	}
	return client, nil
}

// NewVolcengineStorageAPI resolves a branch gateway and service role key for
// the lifetime of one Storage command. Credentials are never persisted.
func NewVolcengineStorageAPI(ctx context.Context, api *volcengine.Client, workspaceID, branchID string) (storage.StorageAPI, error) {
	access, err := api.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return storage.StorageAPI{}, err
	}
	return storage.StorageAPI{Fetcher: newVolcengineRemoteClient(access.BaseURL, access.ServiceRoleKey)}, nil
}

func newLocalClient() *fetcher.Fetcher {
	return fetcher.NewServiceGateway(
		utils.Config.Api.ExternalUrl,
		utils.Config.Auth.ServiceRoleKey.Value,
		fetcher.WithHTTPClient(status.NewKongClient()),
		fetcher.WithUserAgent("SupabaseCLI/"+utils.Version),
	)
}

func newRemoteClient(projectRef, token string) *fetcher.Fetcher {
	return fetcher.NewServiceGateway(
		"https://"+utils.GetSupabaseHost(projectRef),
		token,
		fetcher.WithHTTPClient(http.DefaultClient),
		fetcher.WithUserAgent("SupabaseCLI/"+utils.Version),
	)
}

func newVolcengineRemoteClient(baseURL, token string) *fetcher.Fetcher {
	return fetcher.NewServiceGateway(
		baseURL,
		token,
		fetcher.WithHTTPClient(http.DefaultClient),
		fetcher.WithUserAgent("SupabaseCLI/"+utils.Version),
		fetcher.WithRequestEditor(func(req *http.Request) {
			req.Header.Set(volcengine.HeaderFrom, volcengine.RequestSource())
		}),
	)
}
