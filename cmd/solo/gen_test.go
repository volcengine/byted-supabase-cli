package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func TestGenTypesCommandTreeUsesDataPlaneProfile(t *testing.T) {
	command := newGenCommand(commandtree.RuntimeContext{})
	types, _, err := command.Find([]string{"types"})
	require.NoError(t, err)
	require.NotNil(t, types)
	for _, flag := range []string{"workspace-id", "project-ref", "project-id", "branch-id", "linked"} {
		require.Nil(t, types.Flags().Lookup(flag), "gen types leaked flag %s", flag)
	}
}

func TestGenTypesRequiresServiceRoleKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:        "default",
		SupabaseURL: "https://example.supabase.co",
	}))

	command := newGenTypesCommand(commandtree.RuntimeContext{})
	require.ErrorContains(t, command.Execute(), "missing service role key")
}
