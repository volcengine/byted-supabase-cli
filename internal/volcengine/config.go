// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/backend"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/volcengine-go-sdk/service/aidap"
	"github.com/volcengine/volcengine-go-sdk/volcengine/endpoints"
)

const (
	DefaultRegion       = "cn-beijing"
	EnvAccessKeyID      = "VOLCENGINE_ACCESS_KEY"
	EnvSecretAccessKey  = "VOLCENGINE_SECRET_KEY"
	EnvSessionToken     = "VOLCENGINE_SESSION_TOKEN"
	EnvRegion           = "VOLC_REGION"
	EnvEndpoint         = "VOLC_ENDPOINT"
	EnvProfile          = "VOLC_PROFILE"
	EnvLongRegion       = "VOLCENGINE_REGION"
	EnvLongEndpoint     = "VOLCENGINE_ENDPOINT"
	LinkedWorkspaceFile = "vespb-workspace-id"
	LinkedRegionFile    = "vespb-region"
)

var (
	regionOverride  string
	profileOverride string
)

type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Endpoint        string
}

type FileConfig struct {
	Current     string                     `json:"current"`
	Profiles    map[string]*Profile        `json:"profiles"`
	EnableColor bool                       `json:"enableColor"`
	SsoSession  map[string]json.RawMessage `json:"sso-session,omitempty"`
}

type Profile struct {
	Name             string `json:"name"`
	Mode             string `json:"mode"`
	AccessKey        string `json:"access-key"`
	SecretKey        string `json:"secret-key"`
	Region           string `json:"region"`
	Endpoint         string `json:"endpoint"`
	EndpointResolver string `json:"endpoint-resolver,omitempty"`
	UseDualStack     *bool  `json:"use-dual-stack,omitempty"`
	SessionToken     string `json:"session-token"`
	DisableSSL       *bool  `json:"disable-ssl"`
	SsoSessionName   string `json:"sso-session-name"`
	AccountId        string `json:"account-id"`
	RoleName         string `json:"role-name"`
	StsExpiration    int64  `json:"sts-expiration"`
	LoginSession     string `json:"login-session,omitempty"`
}

type ConfigSet struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Regions         []string
	Endpoint        string
}

const (
	ModeAK           = "ak"
	ModeConsoleLogin = "console-login"
)

func LoadConfigFromEnv() (Config, error) {
	cfgSet, err := LoadConfigSetFromEnv()
	if err != nil {
		return Config{}, err
	}
	if len(cfgSet.Regions) == 0 {
		return cfgSet.ConfigForRegion(DefaultRegion), nil
	}
	return cfgSet.ConfigForRegion(cfgSet.Regions[0]), nil
}

func LoadConfigSetFromEnv() (ConfigSet, error) {
	cfg, err := loadConfig()
	if err != nil {
		return ConfigSet{}, err
	}
	return configSetFromConfig(cfg)
}

func loadConfig() (Config, error) {
	cfg := Config{
		AccessKeyID:     strings.TrimSpace(os.Getenv(EnvAccessKeyID)),
		SecretAccessKey: strings.TrimSpace(os.Getenv(EnvSecretAccessKey)),
		SessionToken:    strings.TrimSpace(os.Getenv(EnvSessionToken)),
		Region:          RegionSetting(),
		Endpoint:        firstEnv(EnvLongEndpoint, EnvEndpoint),
	}
	// An installed control-plane backend (e.g. a proxy distribution) may supply its
	// own credentials. When it reports the config handled, skip the normal AK/SK +
	// profile resolution below.
	if b := backend.Get(); b != nil {
		out, handled, err := b.PrepareCredentials(backend.Credentials{
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			SessionToken:    cfg.SessionToken,
			Region:          cfg.Region,
			Endpoint:        cfg.Endpoint,
		})
		if err != nil {
			return Config{}, err
		}
		if handled {
			cfg.AccessKeyID = out.AccessKeyID
			cfg.SecretAccessKey = out.SecretAccessKey
			cfg.SessionToken = out.SessionToken
			cfg.Region = out.Region
			cfg.Endpoint = out.Endpoint
			return cfg, nil
		}
	}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		if cfg.AccessKeyID == "" {
			return Config{}, errors.Errorf("missing %s environment variable", EnvAccessKeyID)
		}
		if cfg.SecretAccessKey == "" {
			return Config{}, errors.Errorf("missing %s environment variable", EnvSecretAccessKey)
		}
		return cfg, nil
	}
	fileConfig, profileName, profile, err := LoadSelectedProfile()
	if err != nil {
		return Config{}, err
	}
	if profile == nil {
		return Config{}, errors.Errorf("missing Volcengine credentials. Run `supabase configure set --access-key <key> --secret-key <secret> --region <region>` or set %s and %s.", EnvAccessKeyID, EnvSecretAccessKey)
	}
	switch strings.ToLower(strings.TrimSpace(profile.Mode)) {
	case "", ModeAK:
		cfg.AccessKeyID = strings.TrimSpace(profile.AccessKey)
		cfg.SecretAccessKey = strings.TrimSpace(profile.SecretKey)
		cfg.SessionToken = strings.TrimSpace(profile.SessionToken)
	case ModeConsoleLogin:
		creds, err := EnsureValidLoginToken(fileConfig, profileName)
		if err != nil {
			return Config{}, err
		}
		cfg.AccessKeyID = creds.AccessKeyID
		cfg.SecretAccessKey = creds.SecretAccessKey
		cfg.SessionToken = creds.SessionToken
	default:
		return Config{}, errors.Errorf("unsupported Volcengine profile mode %q. Only AK/SK and console-login profiles are supported.", profile.Mode)
	}
	if cfg.Region == "" {
		cfg.Region = strings.TrimSpace(profile.Region)
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = strings.TrimSpace(profile.Endpoint)
	}
	if cfg.AccessKeyID == "" {
		return Config{}, errors.New("missing access-key in Volcengine profile")
	}
	if cfg.SecretAccessKey == "" {
		return Config{}, errors.New("missing secret-key in Volcengine profile")
	}
	return cfg, nil
}

func configSetFromConfig(cfg Config) (ConfigSet, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = DefaultRegion
	}
	if err := ValidateRegion(region); err != nil {
		return ConfigSet{}, err
	}
	return ConfigSet{
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		SessionToken:    cfg.SessionToken,
		Regions:         []string{region},
		Endpoint:        cfg.Endpoint,
	}, nil
}

func VolcengineConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Errorf("failed to get $HOME directory: %w", err)
	}
	return filepath.Join(home, ".volcengine", "config.json"), nil
}

func LoadFileConfig() (FileConfig, error) {
	path, err := VolcengineConfigFilePath()
	if err != nil {
		return FileConfig{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return FileConfig{Current: "default", Profiles: map[string]*Profile{}}, nil
	}
	if err != nil {
		return FileConfig{}, errors.Errorf("failed to read Volcengine config: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return FileConfig{Current: "default", Profiles: map[string]*Profile{}}, nil
	}
	var cfg FileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return FileConfig{}, errors.Errorf("failed to parse Volcengine config: %w", err)
	}
	if cfg.Current == "" {
		cfg.Current = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	return cfg, nil
}

func SaveFileConfig(cfg FileConfig) error {
	path, err := VolcengineConfigFilePath()
	if err != nil {
		return err
	}
	if cfg.Current == "" {
		cfg.Current = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Errorf("failed to create Volcengine config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode Volcengine config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return errors.Errorf("failed to create temporary Volcengine config: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Errorf("failed to write temporary Volcengine config: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Errorf("failed to chmod temporary Volcengine config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return errors.Errorf("failed to close temporary Volcengine config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return errors.Errorf("failed to save Volcengine config: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return errors.Errorf("failed to chmod Volcengine config: %w", err)
	}
	return nil
}

func LoadCurrentProfile() (*Profile, error) {
	_, _, profile, err := LoadSelectedProfile()
	return profile, err
}

func LoadSelectedProfile() (FileConfig, string, *Profile, error) {
	cfg, err := LoadFileConfig()
	if err != nil {
		return FileConfig{}, "", nil, err
	}
	profileName := profileOverride
	if profileName == "" {
		profileName = firstEnv(EnvProfile)
	}
	if profileName == "" {
		profileName = cfg.Current
	}
	if profileName == "" {
		profileName = "default"
	}
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return cfg, profileName, nil, nil
	}
	return cfg, profileName, profile, nil
}

func LoadConfigSetFromEnvWithLinkedRegion(fsys afero.Fs) (ConfigSet, error) {
	cfgSet, err := LoadConfigSetFromEnv()
	if err != nil {
		return ConfigSet{}, err
	}
	if HasRegionOverride() {
		return cfgSet, nil
	}
	region, err := LoadLinkedRegion(fsys)
	if err != nil {
		return ConfigSet{}, err
	}
	if region == "" {
		return cfgSet, nil
	}
	if err := ValidateRegion(region); err != nil {
		return ConfigSet{}, errors.Errorf("invalid linked region: %w", err)
	}
	cfgSet.Regions = []string{region}
	return cfgSet, nil
}

func (c ConfigSet) ConfigForRegion(region string) Config {
	cfg := Config{
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
		SessionToken:    c.SessionToken,
		Region:          region,
		Endpoint:        c.Endpoint,
	}
	return cfg
}

func (c ConfigSet) WithPreferredRegion(region string) ConfigSet {
	region = strings.TrimSpace(region)
	if region == "" {
		return c
	}
	regions := []string{region}
	seen := map[string]struct{}{region: {}}
	for _, current := range c.Regions {
		if _, ok := seen[current]; ok {
			continue
		}
		regions = append(regions, current)
		seen[current] = struct{}{}
	}
	c.Regions = regions
	return c
}

func ConfiguredRegions() []string {
	regions := []string{}
	seen := map[string]struct{}{}
	if region := RegionSetting(); region != "" {
		regions = append(regions, region)
		seen[region] = struct{}{}
	}
	cfg, err := LoadFileConfig()
	if err != nil {
		return regions
	}
	for _, profile := range cfg.Profiles {
		if profile == nil {
			continue
		}
		region := strings.TrimSpace(profile.Region)
		if region == "" {
			continue
		}
		if _, ok := seen[region]; ok {
			continue
		}
		regions = append(regions, region)
		seen[region] = struct{}{}
	}
	return regions
}

func SetRegionOverride(region string) {
	regionOverride = strings.TrimSpace(region)
}

func SetProfileOverride(profile string) {
	profileOverride = strings.TrimSpace(profile)
}

func ValidateRegion(region string) error {
	region = strings.TrimSpace(region)
	if region == "" {
		return errors.New("region cannot be empty")
	}
	if _, err := endpoints.NewStandardEndpointResolver().EndpointFor(aidap.ServiceName, region); err != nil {
		return errors.Errorf("invalid Volcengine region %q: %w", region, err)
	}
	return nil
}

func HasRegionOverride() bool {
	return regionOverride != ""
}

func RegionSetting() string {
	if regionOverride != "" {
		return regionOverride
	}
	if region := firstEnv(EnvLongRegion, EnvRegion); region != "" {
		return region
	}
	profile, err := LoadCurrentProfile()
	if err == nil && profile != nil {
		return strings.TrimSpace(profile.Region)
	}
	return ""
}

func LoadLinkedRegion(fsys afero.Fs) (string, error) {
	regionBytes, err := afero.ReadFile(fsys, filepath.Join(utils.TempDir, LinkedRegionFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.Errorf("failed to load linked Volcengine region: %w", err)
	}
	return string(bytes.TrimSpace(regionBytes)), nil
}

func LoadLinkedWorkspaceID(fsys afero.Fs) (string, error) {
	workspaceIDBytes, err := afero.ReadFile(fsys, filepath.Join(utils.TempDir, LinkedWorkspaceFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", errors.Errorf("failed to load linked Volcengine workspace id: %w", err)
	}
	return string(bytes.TrimSpace(workspaceIDBytes)), nil
}

func HasCredentialsEnv() bool {
	_, err := loadConfig()
	return err == nil
}

func HasAccessKeysEnv() bool {
	return os.Getenv(EnvAccessKeyID) != "" && os.Getenv(EnvSecretAccessKey) != ""
}

func HasNoProfilesAndNoCredentialsEnv() (bool, error) {
	if strings.TrimSpace(os.Getenv(EnvAccessKeyID)) != "" || strings.TrimSpace(os.Getenv(EnvSecretAccessKey)) != "" {
		return false, nil
	}
	cfg, err := LoadFileConfig()
	if err != nil {
		return false, err
	}
	for _, profile := range cfg.Profiles {
		if profile != nil {
			return false, nil
		}
	}
	return true, nil
}

func RequireAccessKeysEnv() error {
	_, err := LoadConfigSetFromEnv()
	return err
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
