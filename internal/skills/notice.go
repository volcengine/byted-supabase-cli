// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"fmt"
	"sync/atomic"

	"github.com/volcengine/byted-supabase-cli/distribution"
)

// StaleNotice signals that the locally installed skill version no longer matches
// the running CLI binary. Current is the last synced version, Target is the
// running binary version.
type StaleNotice struct {
	Current string
	Target  string
}

// Message is a single-line, agent-parseable hint pointing at the fix command.
func (n *StaleNotice) Message() string {
	return fmt.Sprintf(
		"The %s skill (synced for v%s) is out of date for %s v%s. Update it with: %s",
		Name(), normalizeVersion(n.Current), cliName(), normalizeVersion(n.Target), InstallCommand(),
	)
}

// InstallCommand returns the user-facing command that (re)installs the skill,
// branded with the distribution's CLI name when one is registered.
func InstallCommand() string { return cliName() + " skills install" }

// CLIName returns the binary name users type, branded with the registered
// distribution when one is installed (else the upstream default). Exported so
// help text assembled at runtime — after the distribution/agent seams are
// installed — prints the name the running distribution actually uses.
func CLIName() string { return cliName() }

// cliName is the binary name users type: the registered distribution's name
// when one is installed, else the upstream default. Kept in sync with the
// root command's Use via the same distribution seam.
func cliName() string {
	if d := distribution.Get(); d != nil {
		if name := d.CLIName(); name != "" {
			return name
		}
	}
	return "byted-supabase-cli"
}

// pending holds the latest stale notice for this process.
var pending atomic.Pointer[StaleNotice]

// SetPending stores a stale notice; pass nil to clear.
func SetPending(n *StaleNotice) { pending.Store(n) }

// GetPending returns the pending stale notice, or nil.
func GetPending() *StaleNotice { return pending.Load() }
