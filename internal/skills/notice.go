// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package skills

import (
	"fmt"
	"sync/atomic"
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
		"The %s skill (synced for v%s) is out of date for byted-supabase-cli v%s. Update it with: byted-supabase-cli skills install",
		Name(), normalizeVersion(n.Current), normalizeVersion(n.Target),
	)
}

// pending holds the latest stale notice for this process.
var pending atomic.Pointer[StaleNotice]

// SetPending stores a stale notice; pass nil to clear.
func SetPending(n *StaleNotice) { pending.Store(n) }

// GetPending returns the pending stale notice, or nil.
func GetPending() *StaleNotice { return pending.Load() }
