package bootstrap

import "github.com/spf13/cobra"

type Options struct {
	NewRoot   func() *cobra.Command
	Configure func(*cobra.Command)
	Assemble  func(*cobra.Command)
}

func BuildRootCommand(options Options) *cobra.Command {
	root := options.NewRoot()
	options.Configure(root)
	options.Assemble(root)
	return root
}
