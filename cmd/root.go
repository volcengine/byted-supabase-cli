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
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/volcengine/byted-supabase-cli/cmd/bootstrap"
	"github.com/volcengine/byted-supabase-cli/cmd/policy"
	"github.com/volcengine/byted-supabase-cli/cmd/runtime"
	commandtree "github.com/volcengine/byted-supabase-cli/cmd/tree"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func init() {
	rootCmd = bootstrap.BuildRootCommand(bootstrap.Options{
		NewRoot:   newRootCommand,
		Configure: configureRootCommand,
		Assemble: func(root *cobra.Command) {
			assembleRootCommand(root, commandtree.DefaultRuntimeContext())
		},
	})
	bootstrap.AttachLifecycle(rootCmd, func(_ context.Context, cmd *cobra.Command, args []string) error {
		return runtime.RunPreflight(rootPersistentPreRunE, cmd, args)
	})
}

const (
	groupQuickStart    = "quick-start"
	groupLocalDev      = "local-dev"
	groupManagementAPI = "management-api"
)

func IsManagementAPI(cmd *cobra.Command) bool {
	return policy.IsManagementAPI(cmd)
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
	return policy.IsManagementCommand(cmd, policy.ClassificationHooks{
		ManagementExtra: isVolcengineManagementCommandExtra,
	})
}

func isVolcengineManagementCommandExtra(cmd *cobra.Command) bool {
	if hasCommandAncestor(cmd, "pages") {
		switch cmd.Name() {
		case "bind", "binding", "create", "deploy", "env-vars", "list", "sync", "unbind", "upload":
			return true
		}
	}
	switch cmd.Parent().Name() {
	case "ai-gateway":
		return true
	case "projects":
		switch cmd.Name() {
		case "api-keys", "create", "create-tags", "delete", "delete-tags", "deletion-protection", "list", "operations", "overview", "rename", "settings", "sleep", "start", "stop", "wake", "workspace-settings":
			return true
		}
	case "branches":
		switch cmd.Name() {
		case "create", "delete", "get", "get-default", "list", "restorable", "restart", "restore", "restore-window", "set-default", "update", "update-studio-login":
			return true
		}
	case "computes":
		switch cmd.Name() {
		case "get", "list", "restart", "settings", "update":
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
	return policy.IsRemoteDataCommand(cmd, policy.ClassificationHooks{
		RemoteDataExtra: isVolcengineRemoteDataPlaneCommandExtra,
	})
}

func isVolcengineRemoteDataPlaneCommandExtra(cmd *cobra.Command) bool {
	if hasCommandAncestor(cmd, "db") {
		switch cmd.Name() {
		case "connection-string", "query", "advisors", "api-config", "get", "set", "dump", "pull":
			return true
		}
	}
	return cmd.Name() == "types" && hasCommandAncestor(cmd, "gen") ||
		isVolcengineInspectCommand(cmd) ||
		isVolcengineStorageCommand(cmd) ||
		isVolcengineFunctionsCommand(cmd) ||
		isVolcengineSecretsCommand(cmd) ||
		isVolcengineAuthCommand(cmd)
}

func isVolcengineStorageCommand(cmd *cobra.Command) bool {
	if cmd.Parent() == nil {
		return false
	}
	if cmd.Parent().Name() == "storage" {
		switch cmd.Name() {
		case "ls", "cp", "rm", "mv":
			return true
		}
	}
	return cmd.Parent().Parent() != nil &&
		cmd.Parent().Parent().Name() == "storage" &&
		cmd.Parent().Name() == "buckets"
}

func isVolcengineSecretsCommand(cmd *cobra.Command) bool {
	if cmd.Parent() == nil || cmd.Parent().Name() != "secrets" {
		return false
	}
	switch cmd.Name() {
	case "list", "set", "unset":
		return true
	default:
		return false
	}
}

func isVolcengineSeedCommand(cmd *cobra.Command) bool {
	return cmd.Parent() != nil && cmd.Parent().Name() == "seed" && cmd.Name() == "buckets"
}

func isVolcengineFunctionsCommand(cmd *cobra.Command) bool {
	if cmd.Parent() == nil || cmd.Parent().Name() != "functions" {
		return false
	}
	switch cmd.Name() {
	case "list", "delete", "download", "deploy":
		return true
	default:
		return false
	}
}

func isVolcengineAuthCommand(cmd *cobra.Command) bool {
	if !hasCommandAncestor(cmd, "auth") {
		return false
	}
	switch cmd.Name() {
	case "get", "set", "list", "add", "remove", "sync":
		return true
	default:
		return false
	}
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

	rootCmd *cobra.Command

	rootAuthenticator Authenticator = NewCommandAuthenticator(nil, nil)
)

func newRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "byted-supabase-cli",
		Short:             "Byted Supabase CLI " + utils.Version,
		Version:           utils.Version,
		PersistentPreRunE: rootPersistentPreRunE,
		SilenceErrors:     true,
	}
}

func Execute() {
	executeRootCommand()
}

func debugEnabled() bool {
	return viper.GetBool("DEBUG")
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

// instantiate new rootCmd is a bit tricky with cobra, but it can be done later with the following
// approach for example: https://github.com/portworx/pxc/tree/master/cmd
func GetRootCmd() *cobra.Command {
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
