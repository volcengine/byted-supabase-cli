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

package utils

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/go-errors/errors"
	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

var (
	githubClient *github.Client
	githubOnce   sync.Once
)

func GetGitHubClient(ctx context.Context) *github.Client {
	githubOnce.Do(func() {
		var client *http.Client
		if token := os.Getenv("GITHUB_TOKEN"); len(token) > 0 {
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: token},
			)
			client = oauth2.NewClient(ctx, ts)
		}
		githubClient = github.NewClient(client)
	})
	return githubClient
}

const (
	CLI_OWNER = "supabase"
	CLI_REPO  = "cli"

	// Update check uses npm's latest dist-tag as the source of truth: the CLI is distributed
	// via npm and binaries are pulled from CDN by postinstall, so the latest npm version is
	// what `npm i -g ...@latest` actually installs.
	npmRegistry = "https://registry.npmjs.org"
	npmPackage  = "@byted-supabase/cli"
)

// GetLatestRelease queries the npm registry's latest dist-tag and returns a "v"-prefixed version
// string (for semver comparison with utils.Version).
func GetLatestRelease(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, npmRegistry+"/"+npmPackage+"/latest", nil)
	if err != nil {
		return "", errors.Errorf("Failed to fetch latest release: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Errorf("Failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("Failed to fetch latest release: status %d", resp.StatusCode)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return "", errors.Errorf("Failed to parse latest release: %w", err)
	}
	if manifest.Version == "" {
		return "", nil
	}
	return "v" + manifest.Version, nil
}
