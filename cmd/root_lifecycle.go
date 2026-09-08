package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	runtimecli "github.com/volcengine/byted-supabase-cli/cmd/runtime"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
	"golang.org/x/term"
)

func rootPersistentPreRunE(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	ctx, _ := signal.NotifyContext(cmd.Context(), os.Interrupt)
	fsys := afero.NewOsFs()
	profileChanged := cmd.Root().PersistentFlags().Lookup("profile") != nil &&
		cmd.Root().PersistentFlags().Lookup("profile").Changed
	runtimecli.ApplyRegion(profileChanged, rootProfile, volcRegion, volcengine.SetProfileOverride, volcengine.SetRegionOverride)

	authContext, err := rootAuthenticator.Authenticate(ctx, AuthRequest{
		Command:     cmd,
		Args:        args,
		FS:          fsys,
		Interactive: authInteractive(),
	})
	if err != nil {
		return err
	}
	ctx = withAuthContext(ctx, authContext)
	cmd.SetContext(ctx)
	isRemote := isVolcengineRemoteDataPlaneCommand(cmd)
	isVolcengine := authContext.Policy == AuthPolicyVolcengine
	if authContext.Policy == AuthPolicyLocal && IsManagementAPI(cmd) {
		if err := promptLogin(ctx, cmd, fsys); err != nil {
			return err
		}
		if cmd.Flags().Lookup("project-ref") != nil && !isVolcengineManagementCommand(cmd) {
			if err := flags.ParseProjectRef(ctx, fsys); err != nil {
				return err
			}
		}
	}
	if err := utils.ChangeWorkDir(fsys); err != nil {
		return err
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

func authInteractive() bool {
	return authInteractiveWithTTY(
		utils.IsAgentMode(),
		term.IsTerminal(int(os.Stdin.Fd())),
		term.IsTerminal(int(os.Stdout.Fd())),
	)
}

func authInteractiveWithTTY(agentMode, stdinTTY, stdoutTTY bool) bool {
	return !agentMode && stdinTTY && stdoutTTY
}
