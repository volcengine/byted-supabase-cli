package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/telemetry"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

func clearTelemetryEnv(t *testing.T) {
	for _, key := range telemetry.EnvSignalPresenceKeys {
		t.Setenv(key, "")
	}
	for _, key := range telemetry.EnvSignalValueKeys {
		t.Setenv(key, "")
	}
}

func TestCommandAnalyticsContext(t *testing.T) {
	root := &cobra.Command{Use: "supabase"}
	var projectRef string
	var password string
	var debug bool
	output := utils.EnumFlag{
		Allowed: []string{"json", "table"},
		Value:   "table",
	}
	child := &cobra.Command{
		Use: "link",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&debug, "debug", false, "")
	child.Flags().StringVar(&projectRef, "project-ref", "", "")
	child.Flags().StringVar(&password, "password", "", "")
	child.Flags().Var(&output, "output", "")
	child.Flags().AddFlag(root.PersistentFlags().Lookup("debug"))
	markFlagTelemetrySafe(child.Flags().Lookup("project-ref"))
	root.AddCommand(child)

	require.NoError(t, root.PersistentFlags().Set("debug", "true"))
	require.NoError(t, child.Flags().Set("project-ref", "proj_123"))
	require.NoError(t, child.Flags().Set("password", "hunter2"))
	require.NoError(t, child.Flags().Set("output", "json"))

	ctx := commandAnalyticsContext(child)

	assert.Equal(t, "link", ctx.Command)
	assert.Equal(t, map[string]any{
		"debug":       true,
		"output":      "json",
		"password":    redactedTelemetryValue,
		"project-ref": "proj_123",
	}, ctx.Flags)
	assert.NotContains(t, ctx.Flags, "linked")
	assert.NotEmpty(t, ctx.RunID)
}

func TestCommandName(t *testing.T) {
	root := &cobra.Command{Use: "supabase"}
	parent := &cobra.Command{Use: "db"}
	child := &cobra.Command{Use: "push"}
	root.AddCommand(parent)
	parent.AddCommand(child)

	assert.Equal(t, "db push", commandName(child))
	assert.Equal(t, "supabase", commandName(root))
}

func TestPagesCommandsAreVolcengineManagementCommands(t *testing.T) {
	root := &cobra.Command{Use: "byted-supabase-cli"}
	pages := &cobra.Command{Use: "pages"}
	listProjects := &cobra.Command{Use: "list"}
	envVars := &cobra.Command{Use: "env-vars"}
	binding := &cobra.Command{Use: "binding"}
	bind := &cobra.Command{Use: "bind"}
	create := &cobra.Command{Use: "create"}
	upload := &cobra.Command{Use: "upload"}
	deploy := &cobra.Command{Use: "deploy"}
	listDeployments := &cobra.Command{Use: "list"}
	sync := &cobra.Command{Use: "sync"}
	unbind := &cobra.Command{Use: "unbind"}
	root.AddCommand(pages)
	pages.AddCommand(listProjects)
	pages.AddCommand(envVars)
	pages.AddCommand(binding)
	pages.AddCommand(bind)
	pages.AddCommand(create)
	pages.AddCommand(upload)
	pages.AddCommand(deploy)
	deploy.AddCommand(listDeployments)
	pages.AddCommand(sync)
	pages.AddCommand(unbind)

	for _, cmd := range []*cobra.Command{listProjects, envVars, binding, bind, create, upload, deploy, listDeployments, sync, unbind} {
		assert.True(t, isVolcengineManagementCommand(cmd), cmd.CommandPath())
	}
}

func TestVolcengineRemoteDataPlaneCommands(t *testing.T) {
	for _, cmd := range []*cobra.Command{
		dbQueryCmd,
		dbConnectionStringCmd,
		dbDumpCmd,
		dbPullCmd,
		dbAdvisorsCmd,
		genTypesCmd,
	} {
		assert.True(t, isVolcengineRemoteDataPlaneCommand(cmd), cmd.Name())
	}
}

func TestTelemetryIsAgent(t *testing.T) {
	t.Run("returns true for agent env", func(t *testing.T) {
		clearTelemetryEnv(t)
		t.Setenv("CLAUDE_CODE", "1")
		utils.AgentMode.Value = "auto"
		t.Cleanup(func() {
			utils.AgentMode.Value = "auto"
		})

		assert.True(t, telemetryIsAgent())
	})

	t.Run("returns false with no agent env", func(t *testing.T) {
		clearTelemetryEnv(t)
		utils.AgentMode.Value = "auto"
		t.Cleanup(func() {
			utils.AgentMode.Value = "auto"
		})

		assert.False(t, telemetryIsAgent())
	})
}

func TestTelemetryEnvSignals(t *testing.T) {
	clearTelemetryEnv(t)
	t.Setenv("CURSOR_AGENT", "1")
	t.Setenv("TERM_PROGRAM", "  iTerm.app  ")

	signals := telemetryEnvSignals()

	assert.Equal(t, true, signals["CURSOR_AGENT"])
	assert.Equal(t, "iTerm.app", signals["TERM_PROGRAM"])
	assert.NotContains(t, signals, "AI_AGENT")
}

func TestEnvSignals(t *testing.T) {
	clearTelemetryEnv(t)
	t.Setenv("AI_AGENT", "  ")
	t.Setenv("TERM_PROGRAM", "  iTerm.app  ")
	t.Setenv("TERM", strings.Repeat("x", 100))

	signals := envSignals([]string{"AI_AGENT"}, []string{"TERM_PROGRAM", "TERM"})

	assert.Equal(t, "iTerm.app", signals["TERM_PROGRAM"])
	assert.Equal(t, strings.Repeat("x", 80), signals["TERM"])
	assert.NotContains(t, signals, "AI_AGENT")
}
