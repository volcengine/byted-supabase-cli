package cmd

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

func TestResolveAuthPolicyInheritsAndOverrides(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	parent := &cobra.Command{Use: "projects"}
	child := &cobra.Command{Use: "list"}
	root.AddCommand(parent)
	parent.AddCommand(child)
	SetAuthPolicy(parent, AuthPolicyVolcengine)

	policy, err := ResolveAuthPolicy(child, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyVolcengine, policy)

	queryParent := &cobra.Command{Use: "db"}
	query := &cobra.Command{Use: "query"}
	query.Flags().String("db-url", "", "")
	root.AddCommand(queryParent)
	queryParent.AddCommand(query)
	SetAuthPolicy(queryParent, AuthPolicyVolcengine)
	require.NoError(t, query.Flags().Set("db-url", "postgres://user:pass@example.com/db"))

	policy, err = ResolveAuthPolicy(query, nil)
	require.NoError(t, err)
	require.Equal(t, AuthPolicyNone, policy)
}

func TestResolveAuthPolicyUsesNoneForDirectDatabaseCommands(t *testing.T) {
	testCases := []struct {
		name string
		path []string
	}{
		{name: "query", path: []string{"db", "query"}},
		{name: "advisors", path: []string{"db", "advisors"}},
		{name: "dump", path: []string{"db", "dump"}},
		{name: "pull", path: []string{"db", "pull"}},
		{name: "inspect db stats", path: []string{"inspect", "db", "db-stats"}},
		{name: "inspect report", path: []string{"inspect", "report"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := newRootCommand()
			root.AddCommand(newDBCommand(defaultRuntimeContext()), newInspectCommand(defaultRuntimeContext()))
			command, _, err := root.Find(testCase.path)
			require.NoError(t, err)
			policy, err := ResolveAuthPolicy(command, nil)
			require.NoError(t, err)
			require.Equal(t, AuthPolicyVolcengine, policy)
			require.NoError(t, command.Flags().Set("db-url", "postgres://user:pass@localhost/db"))

			policy, err = ResolveAuthPolicy(command, nil)
			require.NoError(t, err)
			require.Equal(t, AuthPolicyNone, policy)
		})
	}
}

func TestAuthInteractiveRequiresBothTTYsAndNonAgentMode(t *testing.T) {
	testCases := []struct {
		name      string
		agentMode bool
		stdinTTY  bool
		stdoutTTY bool
		expected  bool
	}{
		{name: "both tty", stdinTTY: true, stdoutTTY: true, expected: true},
		{name: "stdin is not tty", stdinTTY: false, stdoutTTY: true, expected: false},
		{name: "stdout is not tty", stdinTTY: true, stdoutTTY: false, expected: false},
		{name: "agent mode", agentMode: true, stdinTTY: true, stdoutTTY: true, expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, authInteractiveWithTTY(testCase.agentMode, testCase.stdinTTY, testCase.stdoutTTY))
		})
	}
}

func TestVolcengineAuthStrategyAuthenticates(t *testing.T) {
	expected := volcengine.Config{
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		Region:          "cn-beijing",
	}
	strategy := VolcengineAuthStrategy{
		Credentials: func() (volcengine.Config, error) {
			return expected, nil
		},
		Interactive: func() bool { return false },
	}
	authContext, err := strategy.Authenticate(context.Background(), AuthRequest{
		FS:          afero.NewMemMapFs(),
		Policy:      AuthPolicyVolcengine,
		Interactive: false,
	})
	require.NoError(t, err)
	require.Equal(t, AuthPolicyVolcengine, authContext.Policy)
	require.Equal(t, expected, *authContext.VolcengineConfig)
	require.NotNil(t, authContext.VolcengineClient)
}

func TestNoAuthStrategyDoesNotCreateVolcengineClient(t *testing.T) {
	authContext, err := (NoAuthStrategy{}).Authenticate(context.Background(), AuthRequest{
		Policy: AuthPolicyNone,
	})
	require.NoError(t, err)
	require.Equal(t, AuthPolicyNone, authContext.Policy)
	require.Nil(t, authContext.VolcengineClient)
	require.Nil(t, authContext.VolcengineConfig)
}

func TestCommandAuthenticatorResolvesAndInjectsContext(t *testing.T) {
	command := &cobra.Command{Use: "projects"}
	SetAuthPolicy(command, AuthPolicyVolcengine)
	expected := volcengine.Config{
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		Region:          "cn-beijing",
	}
	authenticator := NewCommandAuthenticator(
		nil,
		authStrategyFactory{strategies: map[AuthPolicy]AuthStrategy{
			AuthPolicyVolcengine: VolcengineAuthStrategy{
				Credentials: func() (volcengine.Config, error) {
					return expected, nil
				},
			},
		}},
	)

	ctx := context.Background()
	authContext, err := authenticator.Authenticate(ctx, AuthRequest{
		Command:     command,
		FS:          afero.NewMemMapFs(),
		Interactive: false,
	})
	require.NoError(t, err)
	require.Equal(t, expected, *authContext.VolcengineConfig)
}
