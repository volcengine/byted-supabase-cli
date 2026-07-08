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
	"github.com/volcengine/byted-supabase-cli/backend"
)

const supabaseConfigDirectory = "byted_supabase_config"

type SupabaseProfileConfig struct {
	IsAgentPlan     bool   `json:"is-agent-plan"`
	AgentPlanSeatID string `json:"agent-plan-seat-id,omitempty"`
}

type SupabaseFileConfig map[string]SupabaseProfileConfig

func SupabaseConfigFilePath() (string, error) {
	path, err := VolcengineConfigFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), supabaseConfigDirectory, "supabase_config.json"), nil
}

func LoadSupabaseFileConfig() (SupabaseFileConfig, error) {
	path, err := SupabaseConfigFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return SupabaseFileConfig{}, nil
	}
	if err != nil {
		return nil, errors.Errorf("failed to read Byted Supabase config: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return SupabaseFileConfig{}, nil
	}
	var cfg SupabaseFileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, errors.Errorf("failed to parse Byted Supabase config: %w", err)
	}
	if cfg == nil {
		cfg = SupabaseFileConfig{}
	}
	for name, profile := range cfg {
		profile.AgentPlanSeatID = strings.TrimSpace(profile.AgentPlanSeatID)
		if profile.AgentPlanSeatID != "" {
			profile.IsAgentPlan = true
		}
		cfg[name] = profile
	}
	return cfg, nil
}

func SaveSupabaseFileConfig(cfg SupabaseFileConfig) error {
	path, err := SupabaseConfigFilePath()
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = SupabaseFileConfig{}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Errorf("failed to create Byted Supabase config directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return errors.Errorf("failed to chmod Byted Supabase config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode Byted Supabase config: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "supabase-config-*.tmp")
	if err != nil {
		return errors.Errorf("failed to create temporary Byted Supabase config: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Errorf("failed to write temporary Byted Supabase config: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.Errorf("failed to chmod temporary Byted Supabase config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return errors.Errorf("failed to close temporary Byted Supabase config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return errors.Errorf("failed to save Byted Supabase config: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return errors.Errorf("failed to chmod Byted Supabase config: %w", err)
	}
	return nil
}

func SetSupabaseProfileConfig(profileName string, profile SupabaseProfileConfig) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return errors.New("missing Volcengine profile name")
	}
	profile.AgentPlanSeatID = strings.TrimSpace(profile.AgentPlanSeatID)
	if profile.AgentPlanSeatID != "" {
		profile.IsAgentPlan = true
	}
	cfg, err := LoadSupabaseFileConfig()
	if err != nil {
		return err
	}
	cfg[profileName] = profile
	return SaveSupabaseFileConfig(cfg)
}

func DeleteSupabaseProfileConfig(profileName string) error {
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return nil
	}
	cfg, err := LoadSupabaseFileConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg[profileName]; !ok {
		return nil
	}
	delete(cfg, profileName)
	return SaveSupabaseFileConfig(cfg)
}

func ResolveCreateWorkspaceAgentPlan(params CreateWorkspaceParams) (CreateWorkspaceParams, error) {
	params.AgentPlanSeatID = strings.TrimSpace(params.AgentPlanSeatID)
	if params.IsAgentPlan != nil || params.AgentPlanSeatID != "" {
		if params.AgentPlanSeatID != "" {
			if params.IsAgentPlan != nil && !*params.IsAgentPlan {
				return params, errors.New("is-agent-plan=false cannot be combined with a non-empty agent-plan-seat-id")
			}
			enabled := true
			params.IsAgentPlan = &enabled
		}
		return params, nil
	}
	if !usesPersistentProfileCredentials() {
		return params, nil
	}
	_, profileName, profile, err := LoadSelectedProfile()
	if err != nil {
		return params, err
	}
	if profile == nil {
		return params, nil
	}
	cfg, err := LoadSupabaseFileConfig()
	if err != nil {
		return params, err
	}
	supabaseProfile, ok := cfg[profileName]
	if !ok {
		return params, nil
	}
	if !supabaseProfile.IsAgentPlan && supabaseProfile.AgentPlanSeatID == "" {
		return params, nil
	}
	enabled := supabaseProfile.IsAgentPlan
	params.IsAgentPlan = &enabled
	params.AgentPlanSeatID = strings.TrimSpace(supabaseProfile.AgentPlanSeatID)
	if params.AgentPlanSeatID != "" {
		enabled = true
		params.IsAgentPlan = &enabled
	}
	return params, nil
}

func usesPersistentProfileCredentials() bool {
	if strings.TrimSpace(os.Getenv(EnvAccessKeyID)) != "" ||
		strings.TrimSpace(os.Getenv(EnvSecretAccessKey)) != "" ||
		strings.TrimSpace(os.Getenv(EnvSessionToken)) != "" {
		return false
	}
	if b := backend.Get(); b != nil {
		return b.UsesPersistentProfileCredentials()
	}
	return true
}
