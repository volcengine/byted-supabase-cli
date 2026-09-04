package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	runtimecli "github.com/volcengine/byted-supabase-cli/cmd/runtime"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func rootPersistentPreRunE(cmd *cobra.Command, args []string) error {
	if IsExperimental(cmd) && !viper.GetBool("EXPERIMENTAL") {
		return errors.New("must set the --experimental flag to run this command")
	}
	cmd.SilenceUsage = true
	ctx, _ := signal.NotifyContext(cmd.Context(), os.Interrupt)
	fsys := afero.NewOsFs()
	isRemote := isVolcengineRemoteDataPlaneCommand(cmd)
	isVolcengine := isVolcengineManagementCommand(cmd) || isRemote
	if err := runtimecli.PrepareAuth(ctx, cmd, fsys, IsManagementAPI(cmd), isVolcengine, runtimecli.AuthHooks{
		LoadProfile: utils.LoadProfile,
		PromptLogin: promptLogin,
		ParseProjectRef: func(ctx context.Context, fsys afero.Fs) error {
			if cmd.Flags().Lookup("project-ref") == nil || isVolcengine {
				return nil
			}
			return flags.ParseProjectRef(ctx, fsys)
		},
	}); err != nil {
		return err
	}
	if err := utils.ChangeWorkDir(fsys); err != nil {
		return err
	}
	profileChanged := cmd.Root().PersistentFlags().Lookup("profile") != nil &&
		cmd.Root().PersistentFlags().Lookup("profile").Changed
	runtimecli.ApplyRegion(profileChanged, rootProfile, volcRegion, volcengine.SetProfileOverride, volcengine.SetRegionOverride)
	if IsManagementAPI(cmd) {
		if err := promptLogin(ctx, cmd, fsys); err != nil {
			return err
		}
		if cmd.Flags().Lookup("project-ref") != nil && !isVolcengineManagementCommand(cmd) {
			if err := flags.ParseProjectRef(ctx, fsys); err != nil {
				return err
			}
		}
	}
	if err := runtimecli.ParseDatabase(ctx, cmd, fsys, isRemote); err != nil {
		return err
	}
	runtimecli.ConfigureDebug(viper.GetBool("DEBUG"), isVolcengine, cmd.Root().Short, printVolcengineDebugProfile)
	ctx, service, err := runtimecli.StartTelemetry(ctx, fsys, commandAnalyticsContext(cmd), telemetry.Options{
		Now: time.Now, IsTTY: telemetryIsTTY(), IsCI: telemetryIsCI(),
		IsAgent: telemetryIsAgent(), EnvSignals: telemetryEnvSignals(), CLIName: utils.Version,
	})
	if err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
	}
	if service != nil {
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
	}
	cmd.SetContext(ctx)
	apiHost, err := url.Parse(utils.GetSupabaseAPIHost())
	if err != nil {
		return err
	}
	sentryOpts.Environment = apiHost.Host
	return sentry.Init(sentryOpts)
}
