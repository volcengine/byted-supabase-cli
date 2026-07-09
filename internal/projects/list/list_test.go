package list

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/h2non/gock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/byted-supabase-cli/internal/testing/apitest"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

func TestBuildVolcengineListPage(t *testing.T) {
	makeResult := func(total, count int) volcengine.ListWorkspacesResult {
		return volcengine.ListWorkspacesResult{
			Total:      total,
			Workspaces: make([]volcengine.Workspace, count),
		}
	}

	t.Run("sets next offset when more pages remain", func(t *testing.T) {
		page := buildVolcengineListPage("cn-beijing", makeResult(17, 10), 0, 10)
		assert.Equal(t, volcengineListPage{
			Region:     "cn-beijing",
			Total:      17,
			Count:      10,
			Offset:     0,
			Limit:      10,
			NextOffset: 10,
		}, page)
	})

	t.Run("omits next offset on last page", func(t *testing.T) {
		page := buildVolcengineListPage("cn-beijing", makeResult(17, 7), 10, 10)
		assert.Equal(t, 0, page.NextOffset)
		assert.Equal(t, 7, page.Count)
		assert.Equal(t, 17, page.Total)
	})

	t.Run("omits next offset when all results returned", func(t *testing.T) {
		page := buildVolcengineListPage("cn-beijing", makeResult(17, 17), 0, 0)
		assert.Equal(t, 0, page.NextOffset)
	})

	t.Run("omits next offset on empty result", func(t *testing.T) {
		page := buildVolcengineListPage("cn-beijing", makeResult(17, 0), 20, 10)
		assert.Equal(t, 0, page.NextOffset)
	})
}

func TestVolcengineListOutputEncoding(t *testing.T) {
	output := volcengineListOutput{
		Projects: []volcengineProject{{ReferenceID: "ws-1", Name: "demo"}},
		Count:    1,
		Total:    17,
		Pagination: []volcengineListPage{
			buildVolcengineListPage("cn-beijing", volcengine.ListWorkspacesResult{
				Total:      17,
				Workspaces: make([]volcengine.Workspace, 10),
			}, 0, 10),
		},
	}

	t.Run("json includes totals and pagination", func(t *testing.T) {
		var buf bytes.Buffer
		assert.NoError(t, utils.EncodeOutput(utils.OutputJson, &buf, output))
		var decoded map[string]any
		assert.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
		assert.Equal(t, float64(17), decoded["total"])
		assert.Equal(t, float64(1), decoded["count"])
		assert.Len(t, decoded["projects"], 1)
		pagination, ok := decoded["pagination"].([]any)
		assert.True(t, ok)
		assert.Len(t, pagination, 1)
		page, ok := pagination[0].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, float64(17), page["total"])
		assert.Equal(t, float64(10), page["next_offset"])
	})

	t.Run("toml encodes without error", func(t *testing.T) {
		var buf bytes.Buffer
		assert.NoError(t, utils.EncodeOutput(utils.OutputToml, &buf, output))
		assert.Contains(t, buf.String(), "total = 17")
	})
}

func TestProjectListCommand(t *testing.T) {
	t.Run("lists all projects", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup valid access token
		token := apitest.RandomAccessToken(t)
		t.Setenv("SUPABASE_ACCESS_TOKEN", string(token))
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects").
			Reply(200).
			JSON([]api.V1ProjectResponse{
				{
					Id:               apitest.RandomProjectRef(),
					OrganizationSlug: "combined-fuchsia-lion",
					Name:             "Test Project",
					Region:           "us-west-1",
					CreatedAt:        "2022-04-25T02:14:55.906498Z",
				},
			})
		// Run test
		assert.NoError(t, Run(context.Background(), fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on failure to load token", func(t *testing.T) {
		assert.Error(t, Run(context.Background(), afero.NewMemMapFs()))
	})

	t.Run("throws error on network error", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup valid access token
		token := apitest.RandomAccessToken(t)
		t.Setenv("SUPABASE_ACCESS_TOKEN", string(token))
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects").
			ReplyError(errors.New("network error"))
		// Run test
		assert.Error(t, Run(context.Background(), fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on server unavailable", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup valid access token
		token := apitest.RandomAccessToken(t)
		t.Setenv("SUPABASE_ACCESS_TOKEN", string(token))
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects").
			Reply(500).
			JSON(map[string]string{"message": "unavailable"})
		// Run test
		assert.Error(t, Run(context.Background(), fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})

	t.Run("throws error on malformed json", func(t *testing.T) {
		// Setup in-memory fs
		fsys := afero.NewMemMapFs()
		// Setup valid access token
		token := apitest.RandomAccessToken(t)
		t.Setenv("SUPABASE_ACCESS_TOKEN", string(token))
		// Flush pending mocks after test execution
		defer gock.OffAll()
		gock.New(utils.DefaultApiHost).
			Get("/v1/projects").
			Reply(200).
			JSON(map[string]string{})
		// Run test
		assert.Error(t, Run(context.Background(), fsys))
		// Validate api
		assert.Empty(t, apitest.ListUnmatchedRequests())
	})
}
