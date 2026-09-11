package solo

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	linkstate "github.com/volcengine/byted-supabase-cli/internal/link"
)

const (
	configDirectory = "byted_supabase_solo"
	configFileName  = "config.json"
	envSupabaseURL  = "SUPABASE_URL"
	envAnonKey      = "SUPABASE_ANON_KEY"
	envServiceKey   = "SUPABASE_SERVICE_ROLE_KEY"
)

type Profile struct {
	Name           string `json:"name"`
	SupabaseURL    string `json:"supabase_url"`
	AnonKey        string `json:"anon_key,omitempty"`
	ServiceRoleKey string `json:"service_role_key,omitempty"`
}

type Config struct {
	Current  string             `json:"current"`
	Profiles map[string]Profile `json:"profiles"`
}

type ConfigSource string

const (
	ConfigSourceProfile     ConfigSource = "profile"
	ConfigSourceEnvironment ConfigSource = "environment"
)

type Resolution struct {
	Profile Profile
	State   linkstate.State
	Source  ConfigSource
}

type Resolver struct {
	ProjectDir string
}

func NewResolver(projectDir string) Resolver {
	return Resolver{ProjectDir: projectDir}
}

type projectLinkState struct {
	Type          string `json:"type"`
	CredentialRef string `json:"credential_ref"`
}

func ConfigFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".volcengine", configDirectory), nil
}

func LoadConfig() (Config, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Current: "default", Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return Config{}, errors.Errorf("failed to read the linked-branch config: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Config{Current: "default", Profiles: map[string]Profile{}}, nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, errors.Errorf("failed to parse the linked-branch config: %w", err)
	}
	if cfg.Current == "" {
		cfg.Current = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func SaveProfile(profile Profile) error {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.SupabaseURL = strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/")
	profile.AnonKey = strings.TrimSpace(profile.AnonKey)
	profile.ServiceRoleKey = strings.TrimSpace(profile.ServiceRoleKey)
	if profile.Name == "" {
		return errors.New("missing linked-branch profile name")
	}
	if profile.SupabaseURL == "" {
		return errors.New("missing Supabase URL")
	}
	parsedURL, err := url.Parse(profile.SupabaseURL)
	if err != nil || parsedURL.Scheme != "http" && parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return errors.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	cfg.Profiles[profile.Name] = profile
	cfg.Current = profile.Name
	return saveConfig(cfg)
}

func DeleteProfile(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("missing linked-branch profile name")
	}
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Profiles[name]; !ok {
		return nil
	}
	delete(cfg.Profiles, name)
	if cfg.Current == name {
		cfg.Current = "default"
		if _, ok := cfg.Profiles[cfg.Current]; !ok {
			cfg.Current = ""
		}
	}
	return saveConfig(cfg)
}

func LoadCurrentProfile() (Profile, error) {
	resolution, err := NewResolver(".").Resolve()
	if err != nil {
		return Profile{}, err
	}
	return resolution.Profile, nil
}

func (r Resolver) Resolve() (Resolution, error) {
	state, stateErr := linkstate.LoadStateAtDir(r.ProjectDir)
	if stateErr == nil && state.Type == linkstate.LinkTypeSolo {
		profile, err := profileForState(state)
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{Profile: profile, State: state, Source: ConfigSourceProfile}, nil
	}
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		return Resolution{}, stateErr
	}
	profile, configured, err := LoadEnvironmentProfile()
	if configured {
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{Profile: profile, Source: ConfigSourceEnvironment}, nil
	}
	if stateErr == nil {
		cfg, err := LoadConfig()
		if err != nil {
			return Resolution{}, err
		}
		profile, ok := cfg.Profiles[cfg.Current]
		if ok {
			return Resolution{Profile: profile, Source: ConfigSourceProfile}, nil
		}
		return Resolution{}, errors.Errorf("the current link does not point at a directly linked branch")
	}
	cfg, err := LoadConfig()
	if err == nil {
		if profile, ok := cfg.Profiles[cfg.Current]; ok {
			return Resolution{Profile: profile, Source: ConfigSourceProfile}, nil
		}
	}
	return Resolution{}, errors.New("no linked-branch profile or environment configuration found")
}

func (r Resolver) Validate() error {
	_, err := r.Resolve()
	return err
}

func profileForState(state linkstate.State) (Profile, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Profile{}, err
	}
	if err := validateProfileState(cfg, state); err != nil {
		return Profile{}, err
	}
	profile, ok := cfg.Profiles[strings.TrimSpace(state.CredentialRef)]
	if !ok {
		return Profile{}, errors.Errorf("linked-branch profile %q not found", state.CredentialRef)
	}
	return profile, nil
}

func validateProfileState(cfg Config, state linkstate.State) error {
	if state.Type != linkstate.LinkTypeSolo {
		return errors.Errorf("the link state does not point at a directly linked branch")
	}
	if strings.TrimSpace(state.CredentialRef) == "" {
		return errors.New("the link state is missing credential_ref")
	}
	profile, ok := cfg.Profiles[strings.TrimSpace(state.CredentialRef)]
	if !ok {
		return errors.Errorf("linked-branch profile %q was not found", state.CredentialRef)
	}
	if err := validateProfile(profile); err != nil {
		return errors.Errorf("linked-branch profile %q is invalid: %w", state.CredentialRef, err)
	}
	return nil
}

func LoadEnvironmentProfile() (Profile, bool, error) {
	values := map[string]string{
		envSupabaseURL: strings.TrimSpace(os.Getenv(envSupabaseURL)),
		envAnonKey:     strings.TrimSpace(os.Getenv(envAnonKey)),
		envServiceKey:  strings.TrimSpace(os.Getenv(envServiceKey)),
	}
	configured := false
	for _, value := range values {
		if value != "" {
			configured = true
			break
		}
	}
	if !configured {
		return Profile{}, false, nil
	}
	missing := make([]string, 0, 2)
	if values[envSupabaseURL] == "" {
		missing = append(missing, envSupabaseURL)
	}
	if values[envAnonKey] == "" && values[envServiceKey] == "" {
		missing = append(missing, envAnonKey+" or "+envServiceKey)
	}
	if len(missing) > 0 {
		return Profile{}, true, errors.Errorf("missing required environment variable(s): %s", strings.Join(missing, ", "))
	}
	profile := Profile{
		Name:           "env",
		SupabaseURL:    values[envSupabaseURL],
		AnonKey:        values[envAnonKey],
		ServiceRoleKey: values[envServiceKey],
	}
	if err := validateProfile(profile); err != nil {
		return Profile{}, true, errors.Errorf("the branch environment configuration is invalid: %w", err)
	}
	return profile, true, nil
}

func ValidateEnvironmentProfile() error {
	_, configured, err := LoadEnvironmentProfile()
	if !configured {
		return nil
	}
	return err
}

func ValidateSoloLinkAtDir(projectDir string) error {
	return NewResolver(projectDir).Validate()
}

func ValidateSoloLinkState(state linkstate.State) error {
	cfg, err := LoadConfig()
	if err != nil {
		return errors.Errorf("the linked-branch config is corrupted: %w", err)
	}
	return validateProfileState(cfg, state)
}

func validateProfile(profile Profile) error {
	if strings.TrimSpace(profile.SupabaseURL) == "" {
		return errors.New("missing Supabase URL")
	}
	parsedURL, err := url.Parse(profile.SupabaseURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return errors.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	if strings.TrimSpace(profile.AnonKey) == "" && strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return errors.New("missing anon key or service role key")
	}
	return nil
}

func loadProjectLinkState() (projectLinkState, error) {
	data, err := os.ReadFile(filepath.Join("supabase", ".temp", "link.json"))
	if err != nil {
		return projectLinkState{}, err
	}
	var state projectLinkState
	if err := json.Unmarshal(data, &state); err != nil {
		return projectLinkState{}, err
	}
	return state, nil
}

func saveConfig(cfg Config) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Errorf("failed to create the linked-branch config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode the linked-branch config: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		return errors.Errorf("failed to save the linked-branch config: %w", err)
	}
	return nil
}
