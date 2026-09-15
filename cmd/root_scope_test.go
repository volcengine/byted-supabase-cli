package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestManagementScopeIsInheritedByNestedCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	projects := &cobra.Command{Use: "projects", GroupID: groupManagementAPI}
	settings := &cobra.Command{Use: "settings"}
	compute := &cobra.Command{Use: "compute"}
	root.AddCommand(projects)
	projects.AddCommand(settings)
	settings.AddCommand(compute)

	require.True(t, IsManagementAPI(compute))
	require.True(t, isVolcengineManagementCommand(compute))
}

func TestSeedBucketsIsVolcengineManagementCommand(t *testing.T) {
	root := newRootCommand()
	root.AddCommand(newSeedCommand(defaultRuntimeContext()))
	seed, _, err := root.Find([]string{"seed", "buckets"})
	require.NoError(t, err)

	require.True(t, isVolcengineManagementCommand(seed))
	require.True(t, isVolcengineRemoteDataPlaneCommand(seed))
}
