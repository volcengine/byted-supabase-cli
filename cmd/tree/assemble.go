package commandtree

import "github.com/spf13/cobra"

func InstallGroup(root *cobra.Command, group CommandGroup, ctx RuntimeContext) {
	if group.Group != nil {
		root.AddGroup(group.Group)
	}
	for _, factory := range group.Factories {
		if command := factory(ctx); command != nil {
			root.AddCommand(command)
		}
	}
}

func Assemble(root *cobra.Command, groups []CommandGroup, ctx RuntimeContext) {
	for _, group := range groups {
		InstallGroup(root, group, ctx)
	}
}
