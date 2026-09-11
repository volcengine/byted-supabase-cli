package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

func TestDBCommandTreeUsesDataPlaneCommands(t *testing.T) {
	command := newDBCommand(commandtree.RuntimeContext{})
	for _, path := range [][]string{
		{"query"},
		{"advisors"},
	} {
		child, _, err := command.Find(path)
		require.NoError(t, err)
		require.NotNil(t, child, "missing Solo db command %q", path)
		for _, flag := range []string{"workspace-id", "project-ref", "branch-id", "linked"} {
			require.Nil(t, child.Flags().Lookup(flag), "%s leaked flag %s", child.CommandPath(), flag)
		}
	}
}
