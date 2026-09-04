package policy

import "github.com/spf13/cobra"

func HasAncestor(cmd *cobra.Command, name string) bool {
	for current := cmd.Parent(); current != nil; current = current.Parent() {
		if current.Name() == name {
			return true
		}
	}
	return false
}
