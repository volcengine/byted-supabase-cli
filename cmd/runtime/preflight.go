package runtime

import "github.com/spf13/cobra"

type Preflight func(*cobra.Command, []string) error

func RunPreflight(hook Preflight, cmd *cobra.Command, args []string) error {
	return hook(cmd, args)
}
