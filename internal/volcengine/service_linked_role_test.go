// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"strings"
	"testing"

	"github.com/go-errors/errors"
)

func TestServiceLinkedRoleNotAssociatedError(t *testing.T) {
	err := &ServiceLinkedRoleNotAssociatedError{Region: "cn-shanghai"}

	msg := err.Error()
	if !strings.Contains(msg, AIDAPServiceLinkedRoleName) {
		t.Errorf("message should name the role %q: %s", AIDAPServiceLinkedRoleName, msg)
	}
	wantURL := "https://console.volcengine.com/aidap/region:aidap+cn-shanghai/workspaces?scene=create"
	if !strings.Contains(msg, wantURL) {
		t.Errorf("message should contain the region-scoped console link %q: %s", wantURL, msg)
	}
	if got := err.AuthorizationURL(); got != wantURL {
		t.Errorf("AuthorizationURL() = %q, want %q", got, wantURL)
	}

	// Empty region falls back to the default region in the link.
	def := (&ServiceLinkedRoleNotAssociatedError{}).AuthorizationURL()
	if !strings.Contains(def, "region:aidap+"+DefaultRegion+"/") {
		t.Errorf("empty region should fall back to %q: %s", DefaultRegion, def)
	}

	// The typed error is recoverable through the go-errors wrapping used across the layer.
	var target *ServiceLinkedRoleNotAssociatedError
	if !errors.As(errors.Errorf("create blocked: %w", err), &target) {
		t.Error("ServiceLinkedRoleNotAssociatedError should be recoverable via errors.As through wrapping")
	}
}
