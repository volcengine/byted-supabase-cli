// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package unset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/secrets/list"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

const volcengineSystemSecretHTTPProxyDisableKeepAlive = "FAAS_HTTP_PROXY_DISABLE_KEEPALIVE"

// RunVolcengine deletes user-managed Edge Function secrets through the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID string, args []string) error {
	if len(args) == 0 {
		secrets, err := list.GetSecretDigestsVolcengine(ctx, client, workspaceID, branchID)
		if err != nil {
			return err
		}
		for _, secret := range secrets {
			if !isVolcengineSystemSecret(secret.Name) {
				args = append(args, secret.Name)
			}
		}
	}
	for _, name := range args {
		if isVolcengineSystemSecret(name) {
			return errors.Errorf("cannot unset system-managed function secret %s", name)
		}
	}
	if len(args) == 0 {
		fmt.Println("You have not set any function secrets, nothing to do.")
		return nil
	}
	msg := fmt.Sprintf("Do you want to unset these function secrets?\n • %s\n\n", strings.Join(args, "\n • "))
	if shouldUnset, err := utils.NewConsole().PromptYesNo(ctx, msg, true); err != nil {
		return err
	} else if !shouldUnset {
		return errors.New(context.Canceled)
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return errors.Errorf("failed to encode secrets unset request: %w", err)
	}
	if _, err := access.DoRequest(ctx, http.MethodDelete, "/v1/projects/default/secrets", bytes.NewReader(payload)); err != nil {
		return err
	}
	fmt.Println("Finished " + utils.Aqua("supabase secrets unset") + ".")
	return nil
}

func isVolcengineSystemSecret(name string) bool {
	return strings.HasPrefix(name, "SUPABASE_") || name == volcengineSystemSecretHTTPProxyDisableKeepAlive
}
