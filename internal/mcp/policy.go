// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// Feature groups matching the official groupings of the legacy Python server (only groups with implemented tools).
const (
	featureAccount     = "account"
	featureDatabase    = "database"
	featureDevelopment = "development"
	featureFunctions   = "functions"
	featureBranching   = "branching"
	featureStorage     = "storage"
	featureCompute     = "compute"
	featurePages       = "pages"
	// featureAuth is a fork-specific group (no upstream equivalent) for Supabase Auth
	// configuration: general config, hooks, and third-party providers.
	featureAuth = "auth"
)

// officialFeatures is the set of supported feature group names.
var officialFeatures = map[string]bool{
	featureAccount:     true,
	featureDatabase:    true,
	featureDevelopment: true,
	featureFunctions:   true,
	featureBranching:   true,
	featureStorage:     true,
	featureCompute:     true,
	featurePages:       true,
	featureAuth:        true,
}

// defaultFeatures enables all supported feature groups (including storage) by default.
//
// This fork deliberately diverges from the upstream "storage off by default" convention
// and enables everything; pass an explicit --features subset to narrow the set (replaces,
// does not extend). Derived from officialFeatures so newly added groups are included automatically.
var defaultFeatures = sortedOfficialFeatures()

func sortedOfficialFeatures() []string {
	out := make([]string, 0, len(officialFeatures))
	for f := range officialFeatures {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// legacyFeatures are feature groups declared by the legacy Python server that this fork
// has not implemented. They already mapped to zero tools in the old server (pure
// placeholders). They are "accepted" here only to stay compatible with legacy configs
// during validation, but expose no tools and do not appear in enabledFeatures / health_check.
var legacyFeatures = map[string]bool{
	"docs":      true,
	"debugging": true,
}

var mcpLoginManager = &pendingLoginManager{}

type pendingLoginManager struct {
	mu      sync.Mutex
	pending *pendingLogin
}

type pendingLogin struct {
	Profile   string
	Region    string
	LoginURL  string
	ExpiresAt time.Time
	Status    string
	Error     string
}

type loginRequiredError struct {
	payload string
}

func (e loginRequiredError) Error() string {
	return e.payload
}

func (m *pendingLoginManager) requireLogin() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := volcengine.LoadConfigFromEnv(); err == nil {
		return nil
	}
	now := time.Now()
	if m.pending != nil {
		if m.pending.Status == "succeeded" {
			m.pending = nil
			return nil
		}
		if m.pending.Status == "pending" && now.Before(m.pending.ExpiresAt) {
			return loginRequiredError{payload: m.pendingPayloadLocked()}
		}
		m.pending = nil
	}
	pending, err := volcengine.StartPendingConsoleLogin(context.Background(), volcengine.PendingConsoleLoginParams{
		Profile: "default",
		Region:  volcengine.DefaultConsoleLoginRegion,
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		return err
	}
	current := &pendingLogin{
		Profile:   pending.Profile,
		Region:    pending.Region,
		LoginURL:  pending.LoginURL,
		ExpiresAt: pending.ExpiresAt,
		Status:    "pending",
	}
	m.pending = current
	go m.wait(current, pending.Done)
	return loginRequiredError{payload: m.pendingPayloadLocked()}
}

func (m *pendingLoginManager) wait(current *pendingLogin, done <-chan error) {
	err := <-done
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending != current {
		return
	}
	if err != nil {
		m.pending.Status = "failed"
		m.pending.Error = err.Error()
		return
	}
	m.pending.Status = "succeeded"
}

func (m *pendingLoginManager) pendingPayloadLocked() string {
	payload := map[string]any{
		"error":      "volcengine_login_required",
		"message":    "No Volcengine profile or environment credentials were found. Open login_url in a browser, complete Console Login, then retry the MCP tool call.",
		"profile":    m.pending.Profile,
		"region":     m.pending.Region,
		"login_url":  m.pending.LoginURL,
		"expires_at": m.pending.ExpiresAt.Format(time.RFC3339),
		"status":     m.pending.Status,
	}
	if m.pending.Error != "" {
		payload["last_error"] = m.pending.Error
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "Volcengine login required. Failed to encode login payload."
	}
	return string(data)
}

// policy is the resolved access policy that determines which tools are exposed.
type policy struct {
	features      map[string]bool
	readOnly      bool
	workspaceRef  string
	disabledTools map[string]bool
	// agentPlanDefault / agentPlanSeatID are the server-configured Agent Plan defaults for
	// newly created workspaces (env AGENT_PLAN / AGENT_PLAN_SEAT_ID). They apply only when the
	// create_workspace caller omitted all agent-plan fields; explicit caller input always wins.
	agentPlanDefault bool
	agentPlanSeatID  string
}

func newPolicy(opts Options) policy {
	features := opts.Features
	if len(features) == 0 {
		features = defaultFeatures
	}
	fm := make(map[string]bool, len(features))
	for _, f := range features {
		if f = strings.TrimSpace(f); f != "" {
			fm[f] = true
		}
	}
	dm := make(map[string]bool, len(opts.DisabledTools))
	for _, d := range opts.DisabledTools {
		if d = strings.TrimSpace(d); d != "" {
			dm[d] = true
		}
	}
	return policy{
		features:         fm,
		readOnly:         opts.ReadOnly,
		workspaceRef:     strings.TrimSpace(opts.WorkspaceRef),
		disabledTools:    dm,
		agentPlanDefault: opts.AgentPlan,
		agentPlanSeatID:  strings.TrimSpace(opts.AgentPlanSeatID),
	}
}

// validateOptions validates the features and disabled-tools values before startup,
// failing fast on any unknown name (mirrors _validate_features / _validate_tools from
// the legacy server) to prevent typos from silently disabling entire tool groups.
// features accepts both the official groups implemented by this fork and the legacy
// placeholder groups (docs/debugging) for backward-config compatibility.
func validateOptions(opts Options) error {
	if bad := unknownNames(opts.Features, knownFeature); len(bad) > 0 {
		return errors.Errorf("unsupported features: %s", strings.Join(bad, ", "))
	}
	tools := allToolNames()
	if bad := unknownNames(opts.DisabledTools, func(n string) bool { return tools[n] }); len(bad) > 0 {
		return errors.Errorf("unsupported disabled-tools: %s", strings.Join(bad, ", "))
	}
	return nil
}

// knownFeature reports whether name is an accepted feature group (an implemented official group or a legacy placeholder).
func knownFeature(name string) bool {
	return officialFeatures[name] || legacyFeatures[name]
}

// allToolNames returns the names of all registered tools (including feature-less
// transport-layer tools) for validating disabled-tools. Backward-compatible aliases are
// included too, so a --disabled-tools entry using a tool's old (pre-rename) name still
// validates and disables that alias route.
func allToolNames() map[string]bool {
	specs := allSpecs()
	names := make(map[string]bool, len(specs))
	for _, t := range specs {
		names[t.meta.name] = true
		for _, a := range t.meta.aliases {
			names[a] = true
		}
	}
	return names
}

// unknownNames returns the entries in values that are not accepted by known, trimmed, deduplicated, and sorted (for use in error messages).
func unknownNames(values []string, known func(string) bool) []string {
	seen := make(map[string]bool)
	var bad []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v == "" || known(v) || seen[v] {
			continue
		}
		seen[v] = true
		bad = append(bad, v)
	}
	sort.Strings(bad)
	return bad
}

// allows reports whether a tool should be exposed under the current policy.
//
// Tools without a feature group are always exposed (only the denylist can remove them);
// this is used for transport-layer tools such as health_check.
func (p policy) allows(name, feature string, mutating bool) bool {
	if p.disabledTools[name] {
		return false
	}
	if p.readOnly && mutating {
		return false
	}
	if feature == "" {
		return true
	}
	if feature == featureAccount && p.workspaceRef != "" {
		return false
	}
	return p.features[feature]
}

// enabledFeatures returns the currently active supported feature groups, sorted.
func (p policy) enabledFeatures() []string {
	out := make([]string, 0, len(p.features))
	for f := range p.features {
		if officialFeatures[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// config is the single credential/region resolution point shared by all tools.
// When no profile or env credentials are present, it returns a pending Console Login URL and waits for the callback in the background.
func (p policy) config(ctx context.Context) (volcengine.Config, error) {
	cfg, err := volcengine.LoadConfigFromEnv()
	if err == nil {
		return cfg, nil
	}
	shouldLogin, detectErr := volcengine.HasNoProfilesAndNoCredentialsEnv()
	if detectErr != nil || !shouldLogin {
		return cfg, err
	}
	return cfg, mcpLoginManager.requireLogin()
}

// readClient builds a client for read operations (without workspace binding).
func (p policy) readClient(ctx context.Context) (*volcengine.Client, error) {
	cfg, err := p.config(ctx)
	if err != nil {
		return nil, err
	}
	return volcengine.NewClient(cfg), nil
}

// writeClient builds a client for write operations with retries disabled to prevent duplicate side effects.
func (p policy) writeClient(ctx context.Context) (*volcengine.Client, error) {
	cfg, err := p.config(ctx)
	if err != nil {
		return nil, err
	}
	return volcengine.NewWriteClient(cfg), nil
}

// client resolves the target workspace and returns a read client bound to it.
func (p policy) client(ctx context.Context, workspaceID string) (*volcengine.Client, string, error) {
	wsID, err := p.resolveWorkspace(workspaceID)
	if err != nil {
		return nil, "", err
	}
	client, err := p.readClient(ctx)
	if err != nil {
		return nil, "", err
	}
	return client, wsID, nil
}

// writeClientFor resolves the target workspace and returns a write client.
func (p policy) writeClientFor(ctx context.Context, workspaceID string) (*volcengine.Client, string, error) {
	wsID, err := p.resolveWorkspace(workspaceID)
	if err != nil {
		return nil, "", err
	}
	client, err := p.writeClient(ctx)
	if err != nil {
		return nil, "", err
	}
	return client, wsID, nil
}

// resolveWorkspace applies the workspace-ref binding rule: when the server is hard-scoped,
// the bound workspace wins and a mismatched caller-supplied id is rejected; otherwise the
// caller is required to supply a workspace id.
func (p policy) resolveWorkspace(input string) (string, error) {
	input = strings.TrimSpace(input)
	if p.workspaceRef != "" {
		if input != "" && input != p.workspaceRef {
			return "", errors.Errorf("workspace %q is not allowed; this server is scoped to %s", input, p.workspaceRef)
		}
		return p.workspaceRef, nil
	}
	if input == "" {
		return "", errors.New("workspace_id is required")
	}
	return input, nil
}
