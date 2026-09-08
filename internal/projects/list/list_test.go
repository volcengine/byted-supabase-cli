package list

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
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
