package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	soloconfig "github.com/volcengine/byted-supabase-cli/internal/solo"
)

func TestInitCommandIsLocalOnly(t *testing.T) {
	command := newInitCommand(commandtree.RuntimeContext{})
	require.Equal(t, "init", command.Name())
	require.Nil(t, command.Flags().Lookup("workspace-id"))
	require.Nil(t, command.Flags().Lookup("branch-id"))
	require.Nil(t, command.Flags().Lookup("linked"))
}

func TestInspectCommandDoesNotExposeManagementFlags(t *testing.T) {
	command := newInspectCommand(commandtree.RuntimeContext{})
	db, _, err := command.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	require.NotNil(t, db)
	require.Nil(t, db.Flags().Lookup("workspace-id"))
	require.Nil(t, db.Flags().Lookup("project-ref"))
	require.Nil(t, db.Flags().Lookup("branch-id"))
	require.Nil(t, db.Flags().Lookup("linked"))
}

func TestInspectIsUnavailableInSoloTree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, soloconfig.SaveProfile(soloconfig.Profile{
		Name:        "default",
		SupabaseURL: "https://example.supabase.co",
	}))
	command := newInspectCommand(commandtree.RuntimeContext{})
	dbStats, _, err := command.Find([]string{"db", "db-stats"})
	require.NoError(t, err)
	require.ErrorContains(t, dbStats.RunE(dbStats, nil), "direct PostgreSQL inspection is unavailable")
}
