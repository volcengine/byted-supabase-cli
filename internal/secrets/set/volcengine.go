// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package set

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

// RunVolcengine sets Edge Function secrets through the selected branch gateway.
func RunVolcengine(ctx context.Context, client *volcengine.Client, workspaceID, branchID, envFilePath string, args []string, fsys afero.Fs) error {
	if err := flags.LoadConfig(fsys); err != nil {
		fmt.Fprintln(utils.GetDebugLogger(), err)
	}
	secrets, err := parseSecrets(envFilePath, fsys, args...)
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		return errors.New("No arguments found. Use --env-file to read from a .env file.")
	}
	access, err := client.ResolvePgMetaAccess(ctx, workspaceID, branchID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(secrets)
	if err != nil {
		return errors.Errorf("failed to encode secrets request: %w", err)
	}
	if _, err := access.DoRequest(ctx, http.MethodPost, "/v1/projects/default/secrets", bytes.NewReader(payload)); err != nil {
		return err
	}
	fmt.Println("Finished " + utils.Aqua("supabase secrets set") + ".")
	return nil
}
