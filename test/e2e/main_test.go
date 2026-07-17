// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

//go:build live_e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

// runMain wraps the provision → run → teardown lifecycle so teardown always
// executes (os.Exit in TestMain would skip deferred calls).
func runMain(m *testing.M) int {
	if err := initHarness(); err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		return 1
	}
	defer os.RemoveAll(h.workDir)

	ctx := context.Background()
	if h.workspaceID != "" {
		fmt.Fprintf(os.Stderr, "e2e: reusing workspace %s (%s set) — it will not be deleted\n", h.workspaceID, envWorkspace)
	} else if err := provisionWorkspace(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "e2e: setup failed:", err)
		return 1
	}

	code := m.Run()

	switch {
	case !h.provisioned:
		// Reused workspace — never delete what we did not create.
	case !h.autoDelete:
		printManualCleanup()
	default:
		if err := deleteWorkspace(ctx); err != nil {
			// With opt-in teardown a leaked workspace must fail the run
			// loudly: it costs money until a sweep catches it.
			fmt.Fprintln(os.Stderr, "e2e: teardown failed:", err)
			if code == 0 {
				code = 1
			}
		} else {
			fmt.Fprintf(os.Stderr, "e2e: deleted workspace %s\n", h.workspaceID)
		}
	}
	return code
}
