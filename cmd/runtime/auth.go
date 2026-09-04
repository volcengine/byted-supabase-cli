package runtime

import (
	"context"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type AuthHooks struct {
	LoadProfile     func(context.Context, afero.Fs) error
	PromptLogin     func(context.Context, *cobra.Command, afero.Fs) error
	ParseProjectRef func(context.Context, afero.Fs) error
}

func PrepareAuth(ctx context.Context, cmd *cobra.Command, fsys afero.Fs, management, volcengine bool, hooks AuthHooks) error {
	if !volcengine && hooks.LoadProfile != nil {
		if err := hooks.LoadProfile(ctx, fsys); err != nil {
			return err
		}
	}
	if management {
		if hooks.PromptLogin != nil {
			if err := hooks.PromptLogin(ctx, cmd, fsys); err != nil {
				return err
			}
		}
		if hooks.ParseProjectRef != nil {
			if err := hooks.ParseProjectRef(ctx, fsys); err != nil {
				return err
			}
		}
	}
	return nil
}
