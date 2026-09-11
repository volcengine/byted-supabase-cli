package solo

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func TestPagesCommandTreeContainsAllDataPlaneCommands(t *testing.T) {
	command := newPagesCommand(commandtree.RuntimeContext{})
	for _, name := range []string{
		"bind", "binding", "create", "deploy",
		"fast", "list", "sync", "unbind", "upload",
	} {
		child, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, child, "missing Solo Pages command %s", name)
		for _, flag := range []string{"workspace-id", "project-ref", "branch-id", "linked"} {
			require.Nil(t, child.Flags().Lookup(flag), "%s leaked flag %s", child.CommandPath(), flag)
		}
	}
	for _, child := range command.Commands() {
		require.NotEqual(t, "env-vars", child.Name(), "unsupported Solo Pages env-vars command must not be exposed")
	}
}

func TestPagesDeployListHasPaginationFlags(t *testing.T) {
	command := newPagesDeploy(commandtree.RuntimeContext{})
	deployList, _, err := command.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, deployList)
	require.NotNil(t, deployList.Flags().Lookup("limit"))
	require.NotNil(t, deployList.Flags().Lookup("offset"))
}

func TestValidatePagesPagination(t *testing.T) {
	require.NoError(t, validatePagesPagination(10, 0))
	require.NoError(t, validatePagesPagination(10, 20))
	require.ErrorContains(t, validatePagesPagination(0, 0), "--limit must be greater than 0")
	require.ErrorContains(t, validatePagesPagination(10, -1), "--offset must not be negative")
	require.ErrorContains(t, validatePagesPagination(10, 5), "--offset must be a multiple of --limit")
}

func TestRenderDeployPaginationSummary(t *testing.T) {
	var output bytes.Buffer
	result := map[string]any{
		"Total": float64(78),
		"Deploys": []any{
			map[string]any{"DeployId": "1"},
			map[string]any{"DeployId": "2"},
			map[string]any{"DeployId": "3"},
			map[string]any{"DeployId": "4"},
			map[string]any{"DeployId": "5"},
			map[string]any{"DeployId": "6"},
			map[string]any{"DeployId": "7"},
			map[string]any{"DeployId": "8"},
			map[string]any{"DeployId": "9"},
			map[string]any{"DeployId": "10"},
		},
	}

	oldFormat := utils.OutputFormat
	utils.OutputFormat.Value = utils.OutputPretty
	t.Cleanup(func() { utils.OutputFormat = oldFormat })

	renderDeployPaginationSummary(&output, result, 10, 0)
	require.Contains(t, output.String(), "Showing 10 of 78 deployments (offset=0, limit=10).")
	require.Contains(t, output.String(), "68 remaining; use --offset 10 --limit 10 to fetch the next page.")

	output.Reset()
	renderDeployPaginationSummary(&output, map[string]any{
		"Total":   float64(10),
		"Deploys": result["Deploys"],
	}, 10, 0)
	require.Empty(t, output.String())
}
