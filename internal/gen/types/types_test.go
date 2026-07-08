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

package types

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/h2non/gock"
	"github.com/jackc/pgconn"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/testing/apitest"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
	"github.com/volcengine/byted-supabase-cli/pkg/pgtest"
)

func TestGenLocalCommand(t *testing.T) {
	utils.DbId = "test-db"
	utils.Config.Hostname = "localhost"
	utils.Config.Db.Port = 5432

	dbConfig := pgconn.Config{
		Host:     utils.Config.Hostname,
		Port:     utils.Config.Db.Port,
		User:     "admin",
		Password: "password",
	}

	t.Run("generates typescript types", func(t *testing.T) {
		const containerId = "test-pgmeta"
		imageUrl := utils.GetRegistryImageUrl(utils.Config.Studio.PgmetaImage)
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup mock docker
		require.NoError(t, apitest.MockDocker(utils.Docker))
		defer gock.OffAll()
		gock.New(utils.Docker.DaemonHost()).
			Get("/v" + utils.Docker.ClientVersion() + "/containers/" + utils.DbId).
			Reply(http.StatusOK).
			JSON(container.InspectResponse{})
		apitest.MockDockerStart(utils.Docker, imageUrl, containerId)
		require.NoError(t, apitest.MockDockerLogs(utils.Docker, containerId, "hello world\n"))
		// Setup mock postgres
		conn := pgtest.NewConn()
		defer conn.Close(t)
		// Run test
		assert.NoError(t, Run(context.Background(), "", dbConfig, LangTypescript, []string{}, true, "", time.Second, fsys, conn.Intercept))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error when db is not started", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup mock docker
		require.NoError(t, apitest.MockDocker(utils.Docker))
		defer gock.OffAll()
		gock.New(utils.Docker.DaemonHost()).
			Get("/v" + utils.Docker.ClientVersion() + "/containers/" + utils.DbId).
			Reply(http.StatusServiceUnavailable)
		// Run test
		assert.Error(t, Run(context.Background(), "", dbConfig, LangTypescript, []string{}, true, "", time.Second, fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on image fetch failure", func(t *testing.T) {
		utils.Config.Api.Image = "v9"
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup mock docker
		require.NoError(t, apitest.MockDocker(utils.Docker))
		defer gock.OffAll()
		gock.New(utils.Docker.DaemonHost()).
			Get("/v" + utils.Docker.ClientVersion() + "/containers/" + utils.DbId).
			Reply(http.StatusOK).
			JSON(container.InspectResponse{})
		gock.New(utils.Docker.DaemonHost()).
			Get("/v" + utils.Docker.ClientVersion() + "/images").
			Reply(http.StatusServiceUnavailable)
		// Run test
		assert.Error(t, Run(context.Background(), "", dbConfig, LangTypescript, []string{}, true, "", time.Second, fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("generates swift types", func(t *testing.T) {
		const containerId = "test-pgmeta"
		imageUrl := utils.GetRegistryImageUrl(utils.Config.Studio.PgmetaImage)
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup mock docker
		require.NoError(t, apitest.MockDocker(utils.Docker))
		defer gock.OffAll()
		gock.New(utils.Docker.DaemonHost()).
			Get("/v" + utils.Docker.ClientVersion() + "/containers/" + utils.DbId).
			Reply(http.StatusOK).
			JSON(container.InspectResponse{})
		apitest.MockDockerStart(utils.Docker, imageUrl, containerId)
		require.NoError(t, apitest.MockDockerLogs(utils.Docker, containerId, "hello world\n"))
		// Setup mock postgres
		conn := pgtest.NewConn()
		defer conn.Close(t)
		// Run test
		assert.NoError(t, Run(context.Background(), "", dbConfig, LangSwift, []string{}, true, SwiftInternalAccessControl, time.Second, fsys, conn.Intercept))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})
}

func TestGenLinkedCommand(t *testing.T) {
	// Setup valid projectId id
	projectId := apitest.RandomProjectRef()
	// Setup valid access token
	token := apitest.RandomAccessToken(t)
	t.Setenv("SUPABASE_ACCESS_TOKEN", string(token))

	t.Run("generates typescript types", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects/" + projectId + "/types/typescript").
			Reply(200).
			JSON(api.TypescriptResponse{Types: ""})
		// Run test
		assert.NoError(t, Run(context.Background(), projectId, pgconn.Config{}, LangTypescript, []string{}, true, "", time.Second, fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on network failure", func(t *testing.T) {
		errNetwork := errors.New("network error")
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects/" + projectId + "/types/typescript").
			ReplyError(errNetwork)
		// Run test
		err := Run(context.Background(), projectId, pgconn.Config{}, LangTypescript, []string{}, true, "", time.Second, fsys)
		// Validate api
		assert.ErrorIs(t, err, errNetwork)
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on service unavailable", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects/" + projectId + "/types/typescript").
			Reply(http.StatusServiceUnavailable)
		// Run test
		assert.Error(t, Run(context.Background(), projectId, pgconn.Config{}, LangTypescript, []string{}, true, "", time.Second, fsys))
	})
}

func TestGenRemoteCommand(t *testing.T) {
	dbConfig := pgconn.Config{
		Host:     "db.supabase.co",
		Port:     5432,
		User:     "admin",
		Password: "password",
		Database: "postgres",
	}

	t.Run("generates type from remote db", func(t *testing.T) {
		const containerId = "test-pgmeta"
		imageUrl := utils.GetRegistryImageUrl(utils.Config.Studio.PgmetaImage)
		// Setup mock docker
		require.NoError(t, apitest.MockDocker(utils.Docker))
		defer gock.OffAll()
		apitest.MockDockerStart(utils.Docker, imageUrl, containerId)
		require.NoError(t, apitest.MockDockerLogs(utils.Docker, containerId, "hello world\n"))
		// Setup mock postgres
		conn := pgtest.NewConn()
		defer conn.Close(t)
		// Run test
		assert.NoError(t, Run(context.Background(), "", dbConfig, LangTypescript, []string{"public"}, true, "", time.Second, afero.NewMemMapFs(), conn.Intercept))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})
}

func TestRunVolcengineHTTPTypescript(t *testing.T) {
	defer gock.OffAll()
	gock.New("https://branch.example.com:443").
		Get("/postgres/generators/typescript").
		MatchParam("included_schemas", "public,private").
		MatchParam("detect_one_to_one_relationships", "true").
		MatchHeader("apikey", "service-role-key").
		MatchHeader("Authorization", "Bearer service-role-key").
		Reply(http.StatusOK).
		BodyString("export type Database = {}")

	var buf bytes.Buffer
	err := runVolcengineHTTPTypes(context.Background(), volcengine.PgMetaAccess{ServiceRoleKey: "service-role-key"}, "https://branch.example.com:443/postgres/generators/typescript", VolcengineParams{
		Lang:    LangTypescript,
		Schemas: []string{"public", "private"},
	}, &buf)
	require.NoError(t, err)
	assert.Equal(t, "export type Database = {}\n", buf.String())
	assert.Empty(t, apitest.ListUnmatchedRequests())
}

func TestRunVolcengineHTTPGo(t *testing.T) {
	defer gock.OffAll()
	gock.New("https://branch.example.com:443").
		Get("/postgres/generators/go").
		MatchParam("included_schemas", "public").
		MatchHeader("apikey", "service-role-key").
		MatchHeader("Authorization", "Bearer service-role-key").
		Reply(http.StatusOK).
		BodyString("package database")

	var buf bytes.Buffer
	err := runVolcengineHTTPTypes(context.Background(), volcengine.PgMetaAccess{ServiceRoleKey: "service-role-key"}, "https://branch.example.com:443/postgres/generators/go", VolcengineParams{
		Lang:    LangGo,
		Schemas: []string{"public"},
	}, &buf)
	require.NoError(t, err)
	assert.Equal(t, "package database\n", buf.String())
	assert.Empty(t, apitest.ListUnmatchedRequests())
}

func TestRunVolcengineHTTPSwift(t *testing.T) {
	defer gock.OffAll()
	gock.New("https://branch.example.com:443").
		Get("/postgres/generators/swift").
		MatchParam("included_schemas", "public").
		MatchParam("access_control", SwiftPublicAccessControl).
		MatchHeader("apikey", "service-role-key").
		MatchHeader("Authorization", "Bearer service-role-key").
		Reply(http.StatusOK).
		BodyString("public struct Database {}")

	var buf bytes.Buffer
	err := runVolcengineHTTPTypes(context.Background(), volcengine.PgMetaAccess{ServiceRoleKey: "service-role-key"}, "https://branch.example.com:443/postgres/generators/swift", VolcengineParams{
		Lang:               LangSwift,
		Schemas:            []string{"public"},
		SwiftAccessControl: SwiftPublicAccessControl,
	}, &buf)
	require.NoError(t, err)
	assert.Equal(t, "public struct Database {}\n", buf.String())
	assert.Empty(t, apitest.ListUnmatchedRequests())
}
