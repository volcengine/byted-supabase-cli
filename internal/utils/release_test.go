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
	"net/http"
	"testing"

	"github.com/go-errors/errors"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/byted-supabase-cli/internal/testing/apitest"
)

func TestLatestRelease(t *testing.T) {
	t.Run("fetches latest release", func(t *testing.T) {
		// Setup api mock
		defer gock.OffAll()
		gock.New("https://registry.npmjs.org").
			Get("/@byted-supabase/cli/latest").
			Reply(http.StatusOK).
			JSON(map[string]string{"version": "2.0.0"})
		// Run test
		version, err := GetLatestRelease(context.Background())
		// Check error
		assert.NoError(t, err)
		assert.Equal(t, version, "v2.0.0")
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("ignores missing version", func(t *testing.T) {
		// Setup api mock
		defer gock.OffAll()
		gock.New("https://registry.npmjs.org").
			Get("/@byted-supabase/cli/latest").
			Reply(http.StatusOK).
			JSON(map[string]string{})
		// Run test
		version, err := GetLatestRelease(context.Background())
		// Check error
		assert.NoError(t, err)
		assert.Empty(t, version)
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on network error", func(t *testing.T) {
		errNetwork := errors.New("network error")
		// Setup api mock
		defer gock.OffAll()
		gock.New("https://registry.npmjs.org").
			Get("/@byted-supabase/cli/latest").
			ReplyError(errNetwork)
		// Run test
		version, err := GetLatestRelease(context.Background())
		// Check error
		assert.ErrorIs(t, err, errNetwork)
		assert.Empty(t, version)
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})
}
