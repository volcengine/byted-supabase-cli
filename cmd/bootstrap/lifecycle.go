package bootstrap

import (
	"context"

	"github.com/spf13/cobra"
)

type LifecycleHook func(context.Context, *cobra.Command, []string) error

func AttachLifecycle(root *cobra.Command, hook LifecycleHook) {
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return hook(cmd.Context(), cmd, args)
	}
}
