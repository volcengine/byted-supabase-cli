package link

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
)

const linkStateVersion = 1

type LinkType string

const (
	LinkTypeManagement LinkType = "management"
	LinkTypeSolo       LinkType = "solo"
)

type State struct {
	Version       int      `json:"version"`
	Type          LinkType `json:"type"`
	SupabaseURL   string   `json:"supabase_url,omitempty"`
	CredentialRef string   `json:"credential_ref,omitempty"`
	WorkspaceID   string   `json:"workspace_id,omitempty"`
	BranchID      string   `json:"branch_id,omitempty"`
	LinkedAt      string   `json:"linked_at"`
}

var StatePath = filepath.Join(utils.TempDir, "link.json")

func StatePathForDir(projectDir string) string {
	return filepath.Join(projectDir, utils.TempDir, "link.json")
}

func LoadState(fsys afero.Fs) (State, error) {
	data, err := afero.ReadFile(fsys, StatePath)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, os.ErrNotExist
	}
	if err != nil {
		return State{}, errors.Errorf("failed to read link state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, errors.Errorf("failed to parse link state: %w", err)
	}
	if state.Version == 0 {
		state.Version = linkStateVersion
	}
	if state.Type != LinkTypeManagement && state.Type != LinkTypeSolo {
		return State{}, errors.Errorf("invalid link type %q", state.Type)
	}
	return state, nil
}

func LoadStateAtDir(projectDir string) (State, error) {
	data, err := os.ReadFile(StatePathForDir(projectDir))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, os.ErrNotExist
	}
	if err != nil {
		return State{}, errors.Errorf("failed to read link state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, errors.Errorf("failed to parse link state: %w", err)
	}
	if state.Version == 0 {
		state.Version = linkStateVersion
	}
	if state.Type != LinkTypeManagement && state.Type != LinkTypeSolo {
		return State{}, errors.Errorf("invalid link type %q", state.Type)
	}
	return state, nil
}

func IsSolo(fsys afero.Fs) bool {
	state, err := LoadState(fsys)
	return err == nil && state.Type == LinkTypeSolo
}

func IsSoloAtDir(projectDir string) bool {
	state, err := LoadStateAtDir(projectDir)
	return err == nil && state.Type == LinkTypeSolo
}

func SaveSoloState(fsys afero.Fs, supabaseURL, credentialRef string) error {
	supabaseURL = strings.TrimRight(strings.TrimSpace(supabaseURL), "/")
	credentialRef = strings.TrimSpace(credentialRef)
	if supabaseURL == "" {
		return errors.New("missing Supabase URL")
	}
	if credentialRef == "" {
		return errors.New("missing credential reference in the branch link")
	}
	return saveState(fsys, State{
		Version:       linkStateVersion,
		Type:          LinkTypeSolo,
		SupabaseURL:   supabaseURL,
		CredentialRef: credentialRef,
		LinkedAt:      time.Now().UTC().Format(time.RFC3339),
	})
}

func SaveManagementState(fsys afero.Fs, workspaceID, branchID string) error {
	return saveState(fsys, State{
		Version:     linkStateVersion,
		Type:        LinkTypeManagement,
		WorkspaceID: strings.TrimSpace(workspaceID),
		BranchID:    strings.TrimSpace(branchID),
		LinkedAt:    time.Now().UTC().Format(time.RFC3339),
	})
}

func ClearState(fsys afero.Fs) error {
	if err := fsys.Remove(StatePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Errorf("failed to remove link state: %w", err)
	}
	return nil
}

func saveState(fsys afero.Fs, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Errorf("failed to encode link state: %w", err)
	}
	if err := utils.WriteFile(StatePath, append(data, '\n'), fsys); err != nil {
		return err
	}
	return nil
}
