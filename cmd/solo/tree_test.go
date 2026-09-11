package solo

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
)

func TestCommandTreeContainsOnlySoloCommands(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{})

	names := commandNames(root)
	for _, name := range []string{
		"auth", "ai-gateway", "endpoints", "functions", "secrets", "seed", "storage",
		"configure", "db", "gen", "init", "inspect",
	} {
		require.Contains(t, names, name, "missing Solo command %q", name)
	}
	for _, name := range []string{"projects", "branches", "computes", "login", "mcp"} {
		require.NotContains(t, names, name, "management command %q leaked into Solo tree", name)
	}
}

func TestSoloAIGatewayCommandUsesDirectDataPlaneFlags(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{})

	command := findDirectCommand(root, "ai-gateway")
	require.NotNil(t, command)
	require.Equal(t, "data-plane", command.GroupID)

	names := commandNames(command)
	for _, name := range []string{"status", "models", "quota", "usage", "chat", "test"} {
		require.Contains(t, names, name)
	}
	for _, name := range []string{"config", "init"} {
		require.NotContains(t, names, name)
	}
	require.NotContains(t, names, "api-keys")

	completions, _, err := command.Find([]string{"chat", "completions"})
	require.NoError(t, err)
	require.NotNil(t, completions.Flag("token"))
	require.NotNil(t, completions.Flag("token-env"))
	require.Nil(t, completions.Flag("workspace-id"))
	require.Nil(t, completions.Flag("branch-id"))

	quota, _, err := command.Find([]string{"quota"})
	require.NoError(t, err)
	require.NotNil(t, quota)
	require.Contains(t, commandNames(quota), "get")
	require.Contains(t, commandNames(quota), "set-default")
	require.Contains(t, commandNames(quota), "users")
}

func TestSoloConfigureOnlySupportsGet(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{})
	configure := findDirectCommand(root, "configure")
	require.NotNil(t, configure)

	names := commandNames(configure)
	require.Contains(t, names, "get")
	require.NotContains(t, names, "set")
	require.NotContains(t, names, "unset")
}

func TestSoloDatabaseCommandsBelongToDataPlane(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{})

	db := findDirectCommand(root, "db")
	require.NotNil(t, db)
	require.Equal(t, "data-plane", db.GroupID)

	inspect := findDirectCommand(root, "inspect")
	require.NotNil(t, inspect)
	require.Equal(t, "data-plane", inspect.GroupID)
}

func TestSoloToolsBelongToTools(t *testing.T) {
	root := NewCommandTree(commandtree.RuntimeContext{})

	require.Equal(t, "tools", findDirectCommand(root, "gen").GroupID)
	require.Equal(t, "tools", findDirectCommand(root, "init").GroupID)
}

func commandNames(root *cobra.Command) map[string]bool {
	names := make(map[string]bool)
	for _, command := range root.Commands() {
		names[command.Name()] = true
	}
	return names
}

func findDirectCommand(root *cobra.Command, name string) *cobra.Command {
	for _, command := range root.Commands() {
		if command.Name() == name {
			return command
		}
	}
	return nil
}
