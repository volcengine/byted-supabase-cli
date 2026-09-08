package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type AuthPolicy string

const (
	AuthPolicyNone       AuthPolicy = "none"
	AuthPolicyLocal      AuthPolicy = "local"
	AuthPolicyVolcengine AuthPolicy = "volcengine"
)

const authPolicyAnnotationKey = "aidap/auth-policy"

type AuthRequest struct {
	Command     *cobra.Command
	Args        []string
	FS          afero.Fs
	Policy      AuthPolicy
	Interactive bool
}

type AuthContext struct {
	Policy           AuthPolicy
	VolcengineConfig *volcengine.Config
	VolcengineClient *volcengine.Client
}

type AuthStrategy interface {
	Authenticate(context.Context, AuthRequest) (*AuthContext, error)
}

type Authenticator interface {
	Authenticate(context.Context, AuthRequest) (*AuthContext, error)
}

type AuthPolicyResolver interface {
	Resolve(context.Context, *cobra.Command, []string) (AuthPolicy, error)
}

type AuthStrategyFactory interface {
	Create(AuthPolicy) (AuthStrategy, error)
}

type CommandAuthenticator struct {
	resolver AuthPolicyResolver
	factory  AuthStrategyFactory
}

type authContextKey struct{}

type authStrategyFactory struct {
	strategies map[AuthPolicy]AuthStrategy
}

func NewAuthStrategyFactory() AuthStrategyFactory {
	return authStrategyFactory{
		strategies: map[AuthPolicy]AuthStrategy{
			AuthPolicyNone:       NoAuthStrategy{},
			AuthPolicyLocal:      LocalAuthStrategy{},
			AuthPolicyVolcengine: VolcengineAuthStrategy{},
		},
	}
}

func NewCommandAuthenticator(resolver AuthPolicyResolver, factory AuthStrategyFactory) Authenticator {
	if resolver == nil {
		resolver = commandAuthPolicyResolver{}
	}
	if factory == nil {
		factory = NewAuthStrategyFactory()
	}
	return &CommandAuthenticator{resolver: resolver, factory: factory}
}

func (a *CommandAuthenticator) Authenticate(ctx context.Context, request AuthRequest) (*AuthContext, error) {
	policy, err := a.resolver.Resolve(ctx, request.Command, request.Args)
	if err != nil {
		return nil, err
	}
	request.Policy = policy
	strategy, err := a.factory.Create(policy)
	if err != nil {
		return nil, err
	}
	return strategy.Authenticate(ctx, request)
}

func (f authStrategyFactory) Create(policy AuthPolicy) (AuthStrategy, error) {
	strategy, ok := f.strategies[policy]
	if !ok {
		return nil, errors.Errorf("unsupported auth policy %q", policy)
	}
	return strategy, nil
}

type commandAuthPolicyResolver struct{}

func (commandAuthPolicyResolver) Resolve(_ context.Context, cmd *cobra.Command, args []string) (AuthPolicy, error) {
	return resolveAuthPolicy(cmd, args)
}

func ResolveAuthPolicy(cmd *cobra.Command, args []string) (AuthPolicy, error) {
	return resolveAuthPolicy(cmd, args)
}

func withAuthContext(ctx context.Context, authContext *AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, authContext)
}

func AuthContextFrom(ctx context.Context) (*AuthContext, error) {
	authContext, ok := ctx.Value(authContextKey{}).(*AuthContext)
	if !ok || authContext == nil {
		return nil, errors.New("authentication context is not available")
	}
	return authContext, nil
}

func SetAuthPolicy(cmd *cobra.Command, policy AuthPolicy) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[authPolicyAnnotationKey] = string(policy)
}

func authPolicyAnnotation(cmd *cobra.Command) (AuthPolicy, bool) {
	if cmd == nil || cmd.Annotations == nil {
		return "", false
	}
	value, ok := cmd.Annotations[authPolicyAnnotationKey]
	return AuthPolicy(value), ok && value != ""
}

func resolveAuthPolicy(cmd *cobra.Command, args []string) (AuthPolicy, error) {
	if isDirectDatabaseCommand(cmd) {
		return AuthPolicyNone, nil
	}
	for current := cmd; current != nil; current = current.Parent() {
		if policy, ok := authPolicyAnnotation(current); ok {
			return policy, nil
		}
	}
	if isVolcengineManagementCommand(cmd) || isVolcengineRemoteDataPlaneCommand(cmd) {
		return AuthPolicyVolcengine, nil
	}
	if IsManagementAPI(cmd) {
		return AuthPolicyLocal, nil
	}
	_ = args
	return AuthPolicyNone, nil
}

func isDirectDatabaseCommand(cmd *cobra.Command) bool {
	if cmd == nil || (!hasCommandAncestor(cmd, "db") && !hasCommandAncestor(cmd, "inspect")) {
		return false
	}
	flag := cmd.Flags().Lookup("db-url")
	return flag != nil && flag.Changed && flag.Value.String() != ""
}

type NoAuthStrategy struct{}

func (NoAuthStrategy) Authenticate(context.Context, AuthRequest) (*AuthContext, error) {
	return &AuthContext{Policy: AuthPolicyNone}, nil
}

type LocalAuthStrategy struct{}

func (LocalAuthStrategy) Authenticate(ctx context.Context, request AuthRequest) (*AuthContext, error) {
	if request.Policy != AuthPolicyLocal {
		return &AuthContext{Policy: request.Policy}, nil
	}
	if err := utils.LoadProfile(ctx, request.FS); err != nil {
		return nil, err
	}
	return &AuthContext{Policy: AuthPolicyLocal}, nil
}

type VolcengineAuthStrategy struct {
	Credentials func() (volcengine.Config, error)
	Login       func(context.Context) error
	Interactive func() bool
}

func (s VolcengineAuthStrategy) Authenticate(ctx context.Context, request AuthRequest) (*AuthContext, error) {
	resolve := s.Credentials
	if resolve == nil {
		resolve = volcengine.LoadConfigFromEnv
	}
	cfg, err := resolve()
	if err != nil {
		interactive := request.Interactive
		if s.Interactive != nil {
			interactive = s.Interactive()
		}
		if !interactive {
			return nil, err
		}
		login := s.Login
		if login == nil {
			login = func(loginCtx context.Context) error {
				return promptVolcengineLogin(loginCtx)
			}
		}
		if err := login(ctx); err != nil {
			return nil, err
		}
		cfg, err = resolve()
		if err != nil {
			return nil, err
		}
	}
	client := volcengine.NewClient(cfg)
	return &AuthContext{
		Policy:           request.Policy,
		VolcengineConfig: &cfg,
		VolcengineClient: client,
	}, nil
}

func promptVolcengineLogin(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "No Volcengine profile found.")
	fmt.Fprintf(os.Stderr, "You can configure AK/SK with `%s`, or set %s and %s.\n", "byted-supabase-cli configure set --access-key <key> --secret-key <key> --region <region>", volcengine.EnvAccessKeyID, volcengine.EnvSecretAccessKey)
	fmt.Fprintln(os.Stderr, "Alternatively, open the following URL to complete Console Login.")
	fmt.Fprintf(os.Stderr, "No region was provided; using default region %q. You can change it later with `%s`.\n", volcengine.DefaultConsoleLoginRegion, "byted-supabase-cli configure region <region>")
	fmt.Fprintln(os.Stderr, "Press Ctrl+C to cancel.")
	return volcengine.RunConsoleLogin(ctx, volcengine.ConsoleLoginParams{
		Profile: "default",
		Region:  volcengine.DefaultConsoleLoginRegion,
		NoOpen:  true,
	}, os.Stdin, os.Stderr)
}
