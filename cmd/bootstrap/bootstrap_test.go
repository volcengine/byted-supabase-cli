package bootstrap

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestBuildRootCommandBuildsInOrder(t *testing.T) {
	var calls []string
	root := BuildRootCommand(Options{
		NewRoot: func() *cobra.Command {
			calls = append(calls, "new-root")
			return &cobra.Command{Use: "test"}
		},
		Configure: func(*cobra.Command) {
			calls = append(calls, "configure")
		},
		Assemble: func(*cobra.Command) {
			calls = append(calls, "assemble")
		},
	})

	require.Equal(t, "test", root.Name())
	require.Equal(t, []string{"new-root", "configure", "assemble"}, calls)
}
