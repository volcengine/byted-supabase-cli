package policy

import "github.com/spf13/cobra"

type ClassificationHooks struct {
	ManagementExtra func(*cobra.Command) bool
	RemoteDataExtra func(*cobra.Command) bool
}

func IsManagementCommand(cmd *cobra.Command, hooks ClassificationHooks) bool {
	if cmd == nil || cmd.Parent() == nil {
		return false
	}
	if IsManagementAPI(cmd) {
		return true
	}
	return hooks.ManagementExtra != nil && hooks.ManagementExtra(cmd)
}

func IsRemoteDataCommand(cmd *cobra.Command, hooks ClassificationHooks) bool {
	if cmd == nil {
		return false
	}
	if hooks.RemoteDataExtra != nil && hooks.RemoteDataExtra(cmd) {
		return true
	}
	return false
}
