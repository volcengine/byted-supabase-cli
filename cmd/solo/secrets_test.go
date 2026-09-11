package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

func TestSecretsCommandTreeHasNoWorkspaceFlags(t *testing.T) {
	command := newSecretsCommand(commandtree.RuntimeContext{})
	for _, name := range []string{"list", "set", "unset"} {
		child, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, child)
		for _, flag := range []string{"workspace-id", "project-ref", "branch-id", "linked"} {
			require.Nil(t, child.Flags().Lookup(flag), "%s leaked flag %s", child.CommandPath(), flag)
		}
	}
}
