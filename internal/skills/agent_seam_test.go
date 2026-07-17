// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"strings"
	"testing"

	"github.com/volcengine/byted-supabase-cli/agent"
	"github.com/volcengine/byted-supabase-cli/distribution"
)

// stubAgent injects a downstream skill configuration through the agent seam.
type stubAgent struct {
	agent.Base
	skill agent.Skill
}

func (a stubAgent) Skill() agent.Skill { return a.skill }

// rebrandDistribution supplies a downstream CLI name through the distribution seam.
type rebrandDistribution struct{ distribution.Base }

func (rebrandDistribution) CLIName() string { return "bytecloud-supabase-cli" }

func TestSourceAndNamePrecedence(t *testing.T) {
	isolate(t)

	// No agent, no env: upstream defaults.
	agent.Set(nil)
	if Source() != defaultSource || Name() != defaultName {
		t.Fatalf("upstream defaults expected, got source=%q name=%q", Source(), Name())
	}

	// A registered agent overrides the defaults.
	agent.Set(stubAgent{skill: agent.Skill{
		Source:    "https://skills.example.com/bytecloud/samples",
		Name:      "bytecloud-supabase",
		Installer: "@example-scope/skills@latest",
		Registry:  "https://npm.example.com",
	}})
	t.Cleanup(func() { agent.Set(nil) })
	if Source() != "https://skills.example.com/bytecloud/samples" || Name() != "bytecloud-supabase" {
		t.Fatalf("agent override expected, got source=%q name=%q", Source(), Name())
	}
	if Installer() != "@example-scope/skills@latest" {
		t.Fatalf("agent installer override expected, got %q", Installer())
	}
	if Registry() != "https://npm.example.com" {
		t.Fatalf("agent registry override expected, got %q", Registry())
	}

	// Env still wins over the agent (testing/staging escape hatch).
	t.Setenv(envSource, "https://staging.example.com/skills")
	t.Setenv(envName, "staging-skill")
	t.Setenv(envInstaller, "staging-installer")
	t.Setenv(envRegistry, "https://staging-npm.example.com")
	if Source() != "https://staging.example.com/skills" || Name() != "staging-skill" {
		t.Fatalf("env override expected, got source=%q name=%q", Source(), Name())
	}
	if Installer() != "staging-installer" {
		t.Fatalf("env installer override expected, got %q", Installer())
	}
	if Registry() != "https://staging-npm.example.com" {
		t.Fatalf("env registry override expected, got %q", Registry())
	}
}

func TestAgentWithEmptySkillKeepsDefaults(t *testing.T) {
	isolate(t)
	agent.Set(stubAgent{})
	t.Cleanup(func() { agent.Set(nil) })
	if Source() != defaultSource || Name() != defaultName {
		t.Fatalf("zero-value skill must keep upstream defaults, got source=%q name=%q", Source(), Name())
	}
	if Installer() != defaultInstaller {
		t.Fatalf("zero-value skill must keep the default installer, got %q", Installer())
	}
	if Registry() != defaultRegistry {
		t.Fatalf("zero-value skill must keep the default registry, got %q", Registry())
	}
}

func TestStaleNoticeMessageUsesDistributionCLIName(t *testing.T) {
	isolate(t)
	distribution.Set(rebrandDistribution{})
	t.Cleanup(func() { distribution.Set(nil) })

	n := &StaleNotice{Current: "v1.0.0", Target: "1.1.0"}
	msg := n.Message()
	if !strings.Contains(msg, "bytecloud-supabase-cli skills install") {
		t.Fatalf("message %q must point at the distribution's install command", msg)
	}
	if strings.Contains(msg, "byted-supabase-cli ") {
		t.Fatalf("message %q must not leak the upstream CLI name", msg)
	}
	if got, want := InstallCommand(), "bytecloud-supabase-cli skills install"; got != want {
		t.Fatalf("InstallCommand() = %q, want %q", got, want)
	}
}
