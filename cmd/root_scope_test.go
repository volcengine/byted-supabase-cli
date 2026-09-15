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

func TestVolcengineWorkspaceCommandsAcceptProjectRefAlias(t *testing.T) {
	ctx := defaultRuntimeContext()
	commands := []struct {
		name string
		root *cobra.Command
		path []string
	}{
		{"auth", newAuthCommand(ctx), []string{"config", "get"}},
		{"branches", newBranchesCommand(ctx), []string{"list"}},
		{"computes", newComputesCommand(ctx), []string{"list"}},
		{"db", newDBCommand(ctx), []string{"query"}},
		{"endpoints", newEndpointsCommand(ctx), []string{"list"}},
		{"functions", newFunctionsCommand(ctx), []string{"list"}},
		{"gen", newGenCommand(ctx), []string{"types"}},
		{"network-restrictions", newNetworkRestrictionsCommand(ctx), []string{"get"}},
		{"pages", newPagesCommand(ctx), []string{"list"}},
		{"projects", newProjectsCommand(ctx), []string{"list"}},
		{"seed", newSeedCommand(ctx), []string{"buckets"}},
		{"secrets", newSecretsCommand(ctx), []string{"list"}},
		{"storage", newStorageCommand(ctx), []string{"ls"}},
	}

	for _, testCase := range commands {
		t.Run(testCase.name, func(t *testing.T) {
			command, _, err := testCase.root.Find(testCase.path)
			require.NoError(t, err)
			require.NotNil(t, command.Flag("project-ref"))
			require.NotNil(t, command.Flag("workspace-id"))

			require.NoError(t, command.Flags().Set("project-ref", "blush-chili-6f170c0o"))
			require.Equal(t, "blush-chili-6f170c0o", command.Flag("workspace-id").Value.String())
		})
	}
}
