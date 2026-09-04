package policy

import "github.com/spf13/cobra"

const ManagementAPIGroup = "management-api"

func IsManagementAPI(cmd *cobra.Command) bool {
	for current := cmd; current != nil && current != cmd.Root(); current = current.Parent() {
		if current.GroupID == ManagementAPIGroup {
			return true
		}
		if current.GroupID != "" {
			break
		}
	}
	return false
}
