// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const stateFileName = "skills-state.json"

// State records the last successful skill install so version drift can be
// detected cheaply (no network) on subsequent CLI runs.
type State struct {
	Skill     string `json:"skill"`
	Source    string `json:"source"`
	Version   string `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

// statePath returns ~/.supabase/skills-state.json — the same global config dir
// as the access token. Skills are installed globally (-g), so their state is
// global too, not project-relative.
func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".supabase", stateFileName), nil
}

// ReadState loads the skills state. The bool is false when no state file exists
// yet (cold start); a non-nil error means the file exists but is unreadable.
func ReadState() (*State, bool, error) {
	path, err := statePath()
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // path derived from $HOME, not user input
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, err
	}
	return &state, true, nil
}

// WriteState persists the skills state, creating ~/.supabase if needed.
func WriteState(state State) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// ReadSyncedVersion returns the recorded skill version, or false if there is no
// readable state yet.
func ReadSyncedVersion() (string, bool) {
	state, ok, err := ReadState()
	if err != nil || !ok || state.Version == "" {
		return "", false
	}
	return state.Version, true
}

// normalizeVersion strips a leading v/V so versions written from different
// sources (git describe "v1.0.0" vs npm "1.0.0") compare equal.
func normalizeVersion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	return strings.TrimPrefix(s, "V")
}
