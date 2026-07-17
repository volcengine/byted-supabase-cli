// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package cmd

import (
	"strings"
	"testing"

	"github.com/volcengine/byted-supabase-cli/agent"
	"github.com/volcengine/byted-supabase-cli/distribution"
)

// downstreamAgent injects a downstream skill name through the agent seam.
type downstreamAgent struct {
	agent.Base
}

func (downstreamAgent) Skill() agent.Skill {
	return agent.Skill{Source: "https://skills.example.com/bytecloud", Name: "bytecloud-supabase"}
}

// downstreamDistribution supplies a downstream CLI name through the distribution seam.
type downstreamDistribution struct{ distribution.Base }

func (downstreamDistribution) CLIName() string { return "bytecloud-supabase-cli" }

// isolateSkillsEnv clears the skills env overrides so the seam-derived name is
// what the help builders see.
func isolateSkillsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BYTED_SUPABASE_CLI_SKILLS_SOURCE", "")
	t.Setenv("BYTED_SUPABASE_CLI_SKILLS_NAME", "")
}

// TestRefreshSkillsHelpUpstreamDefaults verifies the help text keeps the
// upstream identity when no distribution/agent is registered.
func TestRefreshSkillsHelpUpstreamDefaults(t *testing.T) {
	isolateSkillsEnv(t)
	agent.Set(nil)
	distribution.Set(nil)

	refreshSkillsHelp()

	if !strings.Contains(skillsInstallCmd.Long, "-s byted-supabase ") {
		t.Fatalf("upstream skill name expected in Long, got %q", skillsInstallCmd.Long)
	}
	if !strings.Contains(skillsInstallCmd.Example, "byted-supabase-cli skills install") {
		t.Fatalf("upstream CLI name expected in Example, got %q", skillsInstallCmd.Example)
	}
}

// TestRefreshSkillsHelpRebrandsFromSeams verifies that once the
// distribution/agent seams are installed (as main() does after this package's
// init), refreshSkillsHelp rebuilds every skills help string with the
// downstream skill and binary name — and never leaks the upstream identity or
// the removed skills.volces.com host.
func TestRefreshSkillsHelpRebrandsFromSeams(t *testing.T) {
	isolateSkillsEnv(t)
	agent.Set(downstreamAgent{})
	distribution.Set(downstreamDistribution{})
	t.Cleanup(func() {
		agent.Set(nil)
		distribution.Set(nil)
		refreshSkillsHelp() // restore defaults for other tests
	})

	refreshSkillsHelp()

	texts := map[string]string{
		"skills.Short":    skillsCmd.Short,
		"skills.Long":     skillsCmd.Long,
		"install.Short":   skillsInstallCmd.Short,
		"install.Long":    skillsInstallCmd.Long,
		"install.Example": skillsInstallCmd.Example,
	}
	for field, text := range texts {
		if strings.Contains(text, "byted-supabase-cli ") {
			t.Errorf("%s leaks the upstream CLI name: %q", field, text)
		}
		if strings.Contains(text, "skills.volces.com") {
			t.Errorf("%s leaks the hardcoded skills.volces.com host: %q", field, text)
		}
	}

	if !strings.Contains(skillsInstallCmd.Long, "-s bytecloud-supabase ") {
		t.Errorf("install.Long must name the downstream skill, got %q", skillsInstallCmd.Long)
	}
	if !strings.Contains(skillsInstallCmd.Example, "bytecloud-supabase-cli skills install") {
		t.Errorf("install.Example must use the downstream CLI name, got %q", skillsInstallCmd.Example)
	}
	if !strings.Contains(skillsCmd.Long, "bytecloud-supabase-cli update") {
		t.Errorf("skills.Long must use the downstream CLI name in the update hint, got %q", skillsCmd.Long)
	}
}
