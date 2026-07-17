// Copyright (c) 2021 Supabase, Inc. and contributors
// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT
//
// This file has been modified by ByteDance Ltd. and/or its affiliates.
//
// Original file was released under MIT License, with the full license text
// available at https://github.com/supabase/cli/blob/main/LICENSE.
//
// This modified file is released under the same license.

package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/distribution"
	"github.com/volcengine/byted-supabase-cli/internal/debug"
	"github.com/volcengine/byted-supabase-cli/internal/skills"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/update"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

const (
	groupQuickStart    = "quick-start"
	groupLocalDev      = "local-dev"
	groupManagementAPI = "management-api"
)

func IsManagementAPI(cmd *cobra.Command) bool {
	for cmd != cmd.Root() {
		if cmd.GroupID == groupManagementAPI {
			return true
		}
		// Find the last assigned group
		if len(cmd.GroupID) > 0 {
			break
		}
		cmd = cmd.Parent()
	}
	return false
}

func promptLogin(ctx context.Context, cmd *cobra.Command, fsys afero.Fs) error {
	if isVolcengineManagementCommand(cmd) {
		if err := volcengine.RequireAccessKeysEnv(); err != nil {
			shouldLogin, detectErr := volcengine.HasNoProfilesAndNoCredentialsEnv()
			if detectErr != nil || !shouldLogin {
				return err
			}
			fmt.Fprintln(os.Stderr, "No Volcengine profile found.")
			fmt.Fprintf(os.Stderr, "You can configure AK/SK with `%s`, or set %s and %s.\n", "byted-supabase-cli configure set --access-key <key> --secret-key <secret> --region <region>", volcengine.EnvAccessKeyID, volcengine.EnvSecretAccessKey)
			fmt.Fprintln(os.Stderr, "Alternatively, open the following URL to complete Console Login.")
			fmt.Fprintf(os.Stderr, "No region was provided; using default region %q. You can change it later with `%s`.\n", volcengine.DefaultConsoleLoginRegion, "byted-supabase-cli configure region <region>")
			fmt.Fprintln(os.Stderr, "Press Ctrl+C to cancel.")
			if loginErr := volcengine.RunConsoleLogin(ctx, volcengine.ConsoleLoginParams{
				Profile: "default",
				Region:  volcengine.DefaultConsoleLoginRegion,
				NoOpen:  true,
			}, os.Stdin, os.Stderr); loginErr != nil {
				return loginErr
			}
			return volcengine.RequireAccessKeysEnv()
		}
		return nil
	}
	if _, err := utils.LoadAccessTokenFS(fsys); err == utils.ErrMissingToken {
		utils.CmdSuggestion = fmt.Sprintf("Run %s first.", utils.Aqua("byted-supabase-cli login"))
		return errors.New("You need to be logged-in in order to use Management API commands.")
	} else {
		return err
	}
}

func isVolcengineManagementCommand(cmd *cobra.Command) bool {
	if cmd.Parent() == nil {
		return false
	}
	if hasCommandAncestor(cmd, "pages") {
		switch cmd.Name() {
		case "bind", "binding", "create", "deploy", "env-vars", "list", "sync", "unbind", "upload":
			return true
		}
	}
	switch cmd.Parent().Name() {
	case "projects":
		switch cmd.Name() {
		case "api-keys", "compute-settings", "create", "create-tags", "delete", "delete-tags", "deletion-protection", "list", "operations", "overview", "rename", "start", "stop", "workspace-settings":
			return true
		}
	case "branches":
		switch cmd.Name() {
		case "create", "delete", "get", "get-default", "list", "restorable", "restart", "restore", "restore-window", "set-default", "update", "update-studio-login":
			return true
		}
	case "computes":
		switch cmd.Name() {
		case "get", "list", "update":
			return true
		}
	case "network-restrictions":
		switch cmd.Name() {
		case "create", "delete", "get", "update":
			return true
		}
	case "endpoints":
		switch cmd.Name() {
		case "disable-public", "enable-private", "enable-public", "list":
			return true
		}
	case "storage":
		// Storage object access is served by the selected Volcengine branch.
		switch cmd.Name() {
		case "ls", "cp", "rm", "mv":
			return true
		}
	case "functions":
		// Function management is served by the selected Volcengine branch gateway.
		switch cmd.Name() {
		case "list", "delete", "download", "deploy":
			return true
		}
	case "secrets":
		// Function secrets are scoped to the selected Volcengine branch gateway.
		switch cmd.Name() {
		case "list", "set", "unset":
			return true
		}
	case "config":
		// Auth config commands go through the branch Auth Admin API.
		if cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "auth" {
			return true
		}
	case "hooks":
		// Auth hooks commands go through the branch Auth Admin API.
		if cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "auth" {
			return true
		}
	case "third-party":
		// Auth third-party commands go through the branch Auth Admin API.
		if cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "auth" {
			return true
		}
	case "api-config":
		// DATA API config commands go through the branch PostgREST Admin API.
		if cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "db" {
			return true
		}
	case "buckets":
		return cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "storage"
	case "seed":
		// Standard bucket seeding uploads objects through the branch Storage gateway.
		return cmd.Name() == "buckets"
	case "eips":
		return cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "endpoints" && cmd.Name() == "list"
	case "vpcs", "subnets":
		return cmd.Parent().Parent() != nil && cmd.Parent().Parent().Name() == "endpoints" && cmd.Name() == "list"
	}
	return false
}

func hasCommandAncestor(cmd *cobra.Command, name string) bool {
	for current := cmd.Parent(); current != nil; current = current.Parent() {
		if current.Name() == name {
			return true
		}
	}
	return false
}

func isVolcengineRemoteDataPlaneCommand(cmd *cobra.Command) bool {
	return cmd == dbQueryCmd ||
		cmd == dbConnectionStringCmd ||
		cmd == dbDumpCmd ||
		cmd == dbPullCmd ||
		cmd == dbAdvisorsCmd ||
		cmd == genTypesCmd ||
		isVolcengineInspectCommand(cmd) ||
		cmd == lsCmd ||
		cmd == cpCmd ||
		cmd == rmCmd ||
		cmd == mvCmd ||
		cmd == storageBucketsListCmd ||
		cmd == storageBucketsGetCmd ||
		cmd == storageBucketsCreateCmd ||
		cmd == storageBucketsUpdateCmd ||
		cmd == storageBucketsDeleteCmd ||
		cmd == bucketsCmd ||
		cmd == functionsListCmd ||
		cmd == functionsDeleteCmd ||
		cmd == functionsDownloadCmd ||
		cmd == functionsDeployCmd ||
		cmd == secretsListCmd ||
		cmd == secretsSetCmd ||
		cmd == secretsUnsetCmd ||
		cmd == authConfigGetCmd ||
		cmd == authConfigSetCmd ||
		cmd == authHooksGetCmd ||
		cmd == authHooksSetCmd ||
		cmd == authTPListCmd ||
		cmd == authTPAddCmd ||
		cmd == authTPRemoveCmd ||
		cmd == authTPSyncCmd ||
		cmd == dbAPIConfigGetCmd ||
		cmd == dbAPIConfigSetCmd
}

func printVolcengineDebugProfile() {
	_, profileName, profile, err := volcengine.LoadSelectedProfile()
	if err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
		return
	}
	if profile == nil {
		fmt.Fprintln(os.Stderr, "Using Volcengine profile: <env>")
		return
	}
	region := strings.TrimSpace(profile.Region)
	if region == "" {
		region = "<not set>"
	}
	fmt.Fprintf(os.Stderr, "Using Volcengine profile: %s (%s)\n", profileName, region)
}

var experimental = []*cobra.Command{
	// Volcengine currently has no equivalent API for Supabase network bans.
	// Keep the original command implementation for reference, but do not register
	// or gate it as an experimental command in the Volcengine CLI.
	// bansCmd,
	vanityCmd,
	sslEnforcementCmd,
	genKeysCmd,
	postgresCmd,
	dbDeclarativeCmd,
}

func IsExperimental(cmd *cobra.Command) bool {
	for _, exp := range experimental {
		if cmd == exp || cmd.Parent() == exp {
			return true
		}
	}
	return false
}

var (
	sentryOpts = sentry.ClientOptions{
		Dsn:        utils.SentryDsn,
		Release:    utils.Version,
		ServerName: "<redacted>",
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for performance monitoring.
		// We recommend adjusting this value in production,
		TracesSampleRate: 1.0,
	}

	createTicket bool
	rootProfile  string

	rootCmd = &cobra.Command{
		Use:     "byted-supabase-cli",
		Short:   "Byted Supabase CLI " + utils.Version,
		Version: utils.Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if IsExperimental(cmd) && !viper.GetBool("EXPERIMENTAL") {
				return errors.New("must set the --experimental flag to run this command")
			}
			cmd.SilenceUsage = true
			// Load profile before changing workdir
			ctx, _ := signal.NotifyContext(cmd.Context(), os.Interrupt)
			fsys := afero.NewOsFs()
			isVolcengineRemoteDataPlane := isVolcengineRemoteDataPlaneCommand(cmd)
			isVolcengineCommand := isVolcengineManagementCommand(cmd) || isVolcengineRemoteDataPlane
			if !isVolcengineCommand {
				if err := utils.LoadProfile(ctx, fsys); err != nil {
					return err
				}
			}
			if err := utils.ChangeWorkDir(fsys); err != nil {
				return err
			}
			if profileFlag := cmd.Root().PersistentFlags().Lookup("profile"); profileFlag != nil && profileFlag.Changed {
				volcengine.SetProfileOverride(rootProfile)
			}
			volcengine.SetRegionOverride(volcRegion)
			// Add common flags
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
			// Volcengine remote data-plane commands resolve linked/project targets
			// with AIDAP. Explicit DB URLs retain the original parser.
			if !isVolcengineRemoteDataPlane || cmd.Flags().Changed("db-url") {
				if err := flags.ParseDatabaseConfig(ctx, cmd.Flags(), fsys); err != nil {
					return err
				}
			}
			// Prepare context
			if viper.GetBool("DEBUG") {
				http.DefaultTransport = debug.NewTransport()
				fmt.Fprintln(os.Stderr, cmd.Root().Short)
				if isVolcengineCommand {
					printVolcengineDebugProfile()
				} else {
					fmt.Fprintf(os.Stderr, "Using profile: %s (%s)\n", utils.CurrentProfile.Name, utils.CurrentProfile.ProjectHost)
				}
			}
			isTTY := telemetryIsTTY()
			isCI := telemetryIsCI()
			isAgent := telemetryIsAgent()
			envSignals := telemetryEnvSignals()
			service, err := telemetry.NewService(fsys, telemetry.Options{
				Now:        time.Now,
				IsTTY:      isTTY,
				IsCI:       isCI,
				IsAgent:    isAgent,
				EnvSignals: envSignals,
				CLIName:    utils.Version,
			})
			if err != nil {
				fmt.Fprintln(utils.GetDebugLogger(), err)
			} else {
				ctx = telemetry.WithService(ctx, service)
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
			ctx = telemetry.WithCommandContext(ctx, commandAnalyticsContext(cmd))
			cmd.SetContext(ctx)
			// Setup sentry last to ignore errors from parsing cli flags
			apiHost, err := url.Parse(utils.GetSupabaseAPIHost())
			if err != nil {
				return err
			}
			sentryOpts.Environment = apiHost.Host
			return sentry.Init(sentryOpts)
		},
		SilenceErrors: true,
	}
)

func Execute() {
	defer recoverAndExit()
	// Let an installed distribution reshape the assembled tree (rebrand, swap
	// auth, filter, extend, decorate) before anything runs. No-op by default.
	distribution.Apply(rootCmd)
	// Rebuild the skills command help from the now-installed distribution/agent
	// seams (its init-time text predates main()'s Set calls).
	refreshSkillsHelp()
	startedAt := time.Now()
	executedCmd, err := rootCmd.ExecuteC()
	if executedCmd != nil {
		if service := telemetry.FromContext(executedCmd.Context()); service != nil {
			ensureProjectGroupsCached(executedCmd.Context(), service)
			_ = service.Capture(executedCmd.Context(), telemetry.EventCommandExecuted, map[string]any{
				telemetry.PropExitCode:   exitCode(err),
				telemetry.PropDurationMs: time.Since(startedAt).Milliseconds(),
			}, nil)
			_ = service.Close()
		}
	}
	if err != nil {
		panic(err)
	}
	// Check upgrade last because --version flag is initialised after execute.
	// Skip it entirely when the update command has been pruned (e.g. by a
	// distribution whose release channel is not the Volcengine npm registry):
	// the nag would point at a command that no longer exists, and there is no
	// point in the network fetch or cache write either.
	var version string
	if updateCommandMounted() {
		ctx := rootCmd.Context()
		if executedCmd != nil {
			ctx = executedCmd.Context()
		}
		if version, err = checkUpgrade(ctx, afero.NewOsFs()); err != nil {
			fmt.Fprintln(utils.GetDebugLogger(), err)
		}
	}
	if hint := utils.SuggestClaudePlugin(); hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	if (executedCmd == nil || executedCmd.Name() != "update") && update.IsNewer(version, utils.Version) {
		fmt.Fprintln(os.Stderr, suggestUpgrade(version))
	}
	// Cheap, network-free skill drift check: if the locally installed
	// byted-supabase skill was synced for an older CLI version, hint at
	// `skills install`. Suppressed for the update/skills commands, which sync
	// the skill themselves.
	if !skillsNoticeSuppressed(executedCmd) {
		skills.Init(utils.Version)
		if n := skills.GetPending(); n != nil {
			fmt.Fprintln(os.Stderr, n.Message())
		}
	}
	if len(utils.CmdSuggestion) > 0 {
		fmt.Fprintln(os.Stderr, utils.CmdSuggestion)
	}
}

// skillsNoticeSuppressed reports whether the executed command (or any ancestor)
// is the update or skills command, in which case the stale-skill hint is noise.
func skillsNoticeSuppressed(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "update", "skills":
			return true
		}
	}
	return false
}

// ensureProjectGroupsCached populates the telemetry linked-project cache when
// a project ref is available but no cache exists. This ensures org/project
// PostHog groups are attached to all CLI events, not just those after `byted-supabase-cli link`.
//
// Does not overwrite an existing cache — `byted-supabase-cli link` is the authoritative source.
// Checks auth before calling the API to avoid the log.Fatalln in GetSupabase().
func ensureProjectGroupsCached(ctx context.Context, service *telemetry.Service) {
	ref := flags.ProjectRef
	if ref == "" {
		return
	}
	fsys := afero.NewOsFs()
	if telemetry.HasLinkedProject(fsys) {
		return
	}
	if _, err := utils.LoadAccessTokenFS(fsys); err != nil {
		return
	}
	resp, err := utils.GetSupabase().V1GetProjectWithResponse(ctx, ref)
	if err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
		return
	}
	if resp.JSON200 == nil {
		return
	}
	telemetry.CacheProjectAndIdentifyGroups(*resp.JSON200, service, fsys)
}

func exitCode(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

func checkUpgrade(ctx context.Context, fsys afero.Fs) (string, error) {
	if shouldFetchRelease(fsys) {
		version, err := utils.GetLatestRelease(ctx)
		if exists, _ := afero.DirExists(fsys, utils.SupabaseDirPath); exists {
			// If user is offline, write an empty file to skip subsequent checks
			err = utils.WriteFile(utils.CliVersionPath, []byte(version), fsys)
		}
		return version, err
	}
	version, err := afero.ReadFile(fsys, utils.CliVersionPath)
	if err != nil {
		return "", errors.Errorf("failed to read cli version: %w", err)
	}
	return string(version), nil
}

func shouldFetchRelease(fsys afero.Fs) bool {
	// Always fetch latest release when using --version flag
	if vf := rootCmd.Flag("version"); vf != nil && vf.Changed {
		return true
	}
	if fi, err := fsys.Stat(utils.CliVersionPath); err == nil {
		expiry := fi.ModTime().Add(time.Hour * 10)
		// Skip if last checked is less than 10 hours ago
		return time.Now().After(expiry)
	}
	return true
}

func suggestUpgrade(version string) string {
	const command = "byted-supabase-cli update"
	return fmt.Sprintf(`A new version of Byted Supabase CLI is available: %s (currently installed v%s)
Update with: %s`, utils.Yellow(version), utils.Version, utils.Bold(command))
}

func recoverAndExit() {
	err := recover()
	if err == nil {
		return
	}
	var msg string
	switch err := err.(type) {
	case string:
		msg = err
	case error:
		if !errors.Is(err, context.Canceled) &&
			len(utils.CmdSuggestion) == 0 &&
			!viper.GetBool("DEBUG") {
			utils.CmdSuggestion = utils.SuggestDebugFlag
		}
		if e, ok := err.(*errors.Error); ok && len(utils.Version) == 0 {
			fmt.Fprintln(os.Stderr, string(e.Stack()))
		}
		msg = err.Error()
		if details := volcengine.VolcengineErrorDetailsFromErr(err); !details.Empty() {
			msg = appendVolcengineErrorDetails(msg, details)
		}
	default:
		msg = fmt.Sprintf("%#v", err)
	}
	// Log error to console
	fmt.Fprintln(os.Stderr, utils.Red(msg))
	if len(utils.CmdSuggestion) > 0 {
		fmt.Fprintln(os.Stderr, utils.CmdSuggestion)
	}
	// Report error to sentry
	if createTicket && len(utils.SentryDsn) > 0 {
		sentry.ConfigureScope(addSentryScope)
		eventId := sentry.CurrentHub().Recover(err)
		if eventId != nil && sentry.Flush(2*time.Second) {
			fmt.Fprintln(os.Stderr, "Sent crash report:", *eventId)
			fmt.Fprintln(os.Stderr, "Quote the crash ID above when filing a bug report: https://github.com/volcengine/byted-supabase-cli/issues/new/choose")
		}
	}
	os.Exit(1)
}

func appendVolcengineErrorDetails(msg string, details volcengine.ErrorDetails) string {
	var b strings.Builder
	b.WriteString(msg)
	if details.RequestID != "" {
		fmt.Fprintf(&b, "\nRequest ID: %s", details.RequestID)
	}
	if details.StatusCode != "" {
		fmt.Fprintf(&b, "\nStatus Code: %s", details.StatusCode)
	}
	if details.ErrorCode != "" {
		fmt.Fprintf(&b, "\nError Code: %s", details.ErrorCode)
	}
	if details.Message != "" {
		fmt.Fprintf(&b, "\nMessage: %s", details.Message)
	}
	return b.String()
}

func init() {
	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("SUPABASE")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()
	})

	flags := rootCmd.PersistentFlags()
	flags.Bool("yes", false, "answer yes to all prompts")
	flags.Bool("debug", false, "output debug logs to stderr")
	flags.String("workdir", "", "path to a Supabase project directory")
	flags.Bool("experimental", false, "enable experimental features")
	flags.String("network-id", "", "use the specified docker network instead of a generated one")
	flags.StringVar(&rootProfile, "profile", "supabase", "use a specific profile; for Volcengine commands, overrides the current configure profile")
	flags.Lookup("profile").DefValue = "current"
	flags.StringVar(&volcRegion, "region", "", "Volcengine region for management API requests.")
	flags.VarP(&utils.OutputFormat, "output", "o", "output format of status variables")
	flags.Var(&utils.DNSResolver, "dns-resolver", "lookup domain names using the specified resolver")
	// Volcengine CLI does not support public support-ticket creation yet.
	// Keep the original implementation for future integration, but do not expose the flag.
	// flags.BoolVar(&createTicket, "create-ticket", false, "create a support ticket for any CLI error")
	flags.VarP(&utils.AgentMode, "agent", "", "Override agent detection: yes, no, or auto (default auto)")
	cobra.CheckErr(viper.BindPFlags(flags))
	cobra.CheckErr(flags.MarkHidden("experimental"))
	cobra.CheckErr(flags.MarkHidden("network-id"))
	cobra.CheckErr(flags.MarkHidden("dns-resolver"))

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.AddGroup(&cobra.Group{ID: groupQuickStart, Title: "Quick Start:"})
	rootCmd.AddGroup(&cobra.Group{ID: groupLocalDev, Title: "Local Development:"})
	rootCmd.AddGroup(&cobra.Group{ID: groupManagementAPI, Title: "Management APIs:"})
}

// instantiate new rootCmd is a bit tricky with cobra, but it can be done later with the following
// approach for example: https://github.com/portworx/pxc/tree/master/cmd
func GetRootCmd() *cobra.Command {
	// Apply here too so distributions that inspect or re-mount the tree before
	// calling Execute see the already-customized root. Idempotent.
	distribution.Apply(rootCmd)
	refreshSkillsHelp()
	return rootCmd
}

func addSentryScope(scope *sentry.Scope) {
	serviceImages := utils.Config.GetServiceImages()
	imageToVersion := make(map[string]any, len(serviceImages))
	for _, image := range serviceImages {
		parts := strings.Split(image, ":")
		// Bypasses sentry's IP sanitization rule, ie. 15.1.0.147
		if net.ParseIP(parts[1]) != nil {
			imageToVersion[parts[0]] = "v" + parts[1]
		} else {
			imageToVersion[parts[0]] = parts[1]
		}
	}
	scope.SetContext("Services", imageToVersion)
	scope.SetContext("Config", map[string]any{
		"Image Registry": utils.GetRegistry(),
		"Project ID":     flags.ProjectRef,
	})
}
