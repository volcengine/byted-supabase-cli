package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/cmd/bootstrap"
	"github.com/volcengine/byted-supabase-cli/internal/skills"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/update"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func executeRootCommand() {
	defer bootstrap.RecoverAndExit(bootstrap.ErrorHooks{
		Suggestion:    func() string { return utils.CmdSuggestion },
		SetSuggestion: func(value string) { utils.CmdSuggestion = value },
		Debug:         func() bool { return debugEnabled() },
		Format: func(err error) string {
			details := volcengine.VolcengineErrorDetailsFromErr(err)
			if details.Empty() {
				return utils.Red(err.Error())
			}
			return utils.Red(bootstrap.FormatErrorDetails(err.Error(), details.RequestID, details.StatusCode, details.ErrorCode, details.Message))
		},
		CreateTicket: func() bool { return createTicket },
		SentryDSN:    func() string { return utils.SentryDsn },
		Scope:        addSentryScope,
	})
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
	var version string
	ctx := rootCmd.Context()
	if executedCmd != nil {
		ctx = executedCmd.Context()
	}
	if version, err = checkUpgrade(ctx, afero.NewOsFs()); err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
	}
	if hint := utils.SuggestClaudePlugin(); hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	if (executedCmd == nil || executedCmd.Name() != "update") && update.IsNewer(version, utils.Version) {
		fmt.Fprintln(os.Stderr, suggestUpgrade(version))
	}
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
