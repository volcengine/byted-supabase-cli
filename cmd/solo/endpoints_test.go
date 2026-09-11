package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

func TestEndpointsCommandTreeHasOnlyDataPlaneCommands(t *testing.T) {
	command := newEndpointsCommand(commandtree.RuntimeContext{})
	for _, name := range []string{"list", "check"} {
		child, _, err := command.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, child)
	}
	for _, name := range []string{"enable-public", "disable-public", "enable-private", "eips", "vpcs", "subnets"} {
		for _, child := range command.Commands() {
			require.NotEqual(t, name, child.Name())
		}
	}
}
