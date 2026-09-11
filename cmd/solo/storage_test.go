package solo

import (
	"testing"

	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

func TestStorageCommandTreeHasNoWorkspaceFlags(t *testing.T) {
	storageCommand := newStorageCommand(commandtree.RuntimeContext{})
	for _, path := range [][]string{
		{"ls"},
		{"cp"},
		{"mv"},
		{"rm"},
		{"buckets", "list"},
		{"buckets", "get"},
		{"buckets", "create"},
		{"buckets", "update"},
		{"buckets", "delete"},
	} {
		command, _, err := storageCommand.Find(path)
		require.NoError(t, err)
		require.NotNil(t, command, "missing Solo Storage command %q", path)
		for _, flag := range []string{"workspace-id", "project-ref", "branch-id", "linked"} {
			require.Nil(t, command.Flags().Lookup(flag), "%s leaked flag %s", command.CommandPath(), flag)
		}
	}
}

func TestStorageRecursiveCommandsAreMounted(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{})
	seed, _, err := root.Find([]string{"seed", "buckets"})
	require.NoError(t, err)
	require.NotNil(t, seed)
	for _, path := range [][]string{
		{"storage", "cp"},
		{"storage", "mv"},
		{"storage", "rm"},
	} {
		command, _, err := root.Find(path)
		require.NoError(t, err)
		require.NotNil(t, command)
		require.NotNil(t, command.Flags().Lookup("recursive"), "%s missing recursive flag", command.CommandPath())
	}
}
