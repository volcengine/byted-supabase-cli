package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func StartTelemetry(ctx context.Context, fsys afero.Fs, commandContext telemetry.CommandContext, options telemetry.Options) (context.Context, *telemetry.Service, error) {
	service, err := telemetry.NewService(fsys, options)
	if err != nil {
		return ctx, nil, err
	}
	ctx = telemetry.WithService(ctx, service)
	var stitchOnce sync.Once
	utils.OnGotrueID = func(gotrueID string) {
		if service.NeedsIdentityStitch() {
			stitchOnce.Do(func() {
				if err := service.StitchLogin(gotrueID); err != nil {
					fmt.Fprintln(utils.GetDebugLogger(), err)
				}
			})
		}
	}
	_ = commandContext
	return telemetry.WithCommandContext(ctx, commandContext), service, nil
}
