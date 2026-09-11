package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

func TestFunctionsCommandTreeHasNoWorkspaceFlags(t *testing.T) {
	command := newFunctionsCommand(commandtree.RuntimeContext{})
	for _, name := range []string{"list", "download", "delete", "new"} {
		child, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, child)
		for _, flag := range []string{"workspace-id", "project-ref", "branch-id", "linked"} {
			require.Nil(t, child.Flags().Lookup(flag), "%s leaked flag %s", child.CommandPath(), flag)
		}
	}
	deploy, _, err := command.Find([]string{"deploy"})
	require.NoError(t, err)
	require.NotNil(t, deploy)
}
