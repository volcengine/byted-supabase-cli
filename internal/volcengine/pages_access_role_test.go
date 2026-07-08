// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package volcengine

import (
	"strings"
	"testing"

	"github.com/go-errors/errors"
)

func TestPagesAccessRoleNotAssociatedError(t *testing.T) {
	err := &PagesAccessRoleNotAssociatedError{}

	msg := err.Error()
	if !strings.Contains(msg, dcdnAccessRoleName) {
		t.Errorf("message should name the role %q: %s", dcdnAccessRoleName, msg)
	}

	url := err.AuthorizationURL()
	wantURL := "https://console.volcengine.com/iam/service/attach_custom_role?ServiceName=dcdn&role1=DCDNAccessAIDPRole&policy1_1=DCDNAccessAIDPRolePolicy"
	if url != wantURL {
		t.Errorf("AuthorizationURL() = %q, want %q", url, wantURL)
	}
	if !strings.Contains(msg, url) {
		t.Errorf("message should contain the authorization link %q: %s", url, msg)
	}
	// The page must carry the exact role + policy names so it provisions the right role.
	for _, want := range []string{"attach_custom_role", "role1=" + dcdnAccessRoleName, "policy1_1=" + dcdnAccessRolePolicyName} {
		if !strings.Contains(url, want) {
			t.Errorf("authorization URL %q missing %q", url, want)
		}
	}

	// Recoverable through the go-errors wrapping used across the layer.
	var target *PagesAccessRoleNotAssociatedError
	if !errors.As(errors.Errorf("deploy blocked: %w", err), &target) {
		t.Error("PagesAccessRoleNotAssociatedError should be recoverable via errors.As through wrapping")
	}
}
