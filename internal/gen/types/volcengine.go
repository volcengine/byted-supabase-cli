// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package types

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type VolcengineParams struct {
	WorkspaceID        string
	BranchID           string
	Lang               string
	Schemas            []string
	PostgrestV9Compat  bool
	SwiftAccessControl string
}

func RunVolcengine(ctx context.Context, client *volcengine.Client, params VolcengineParams, w io.Writer) error {
	access, err := client.ResolvePgMetaAccess(ctx, params.WorkspaceID, params.BranchID)
	if err != nil {
		return err
	}
	generatorURL, err := volcengine.BuildPgMetaURL(access.BaseURL, "/postgres/generators/"+params.Lang)
	if err != nil {
		return err
	}
	return runVolcengineHTTPTypes(ctx, access, generatorURL, params, w)
}

func runVolcengineHTTPTypes(ctx context.Context, access volcengine.PgMetaAccess, generatorURL string, params VolcengineParams, w io.Writer) error {
	parsed, err := url.Parse(generatorURL)
	if err != nil {
		return errors.Errorf("failed to parse types generator URL: %w", err)
	}
	schemas := params.Schemas
	if len(schemas) == 0 {
		schemas = utils.RemoveDuplicates(append([]string{"public"}, utils.Config.Api.Schemas...))
	}
	values := parsed.Query()
	values.Set("included_schemas", strings.Join(schemas, ","))
	switch params.Lang {
	case LangTypescript:
		values.Set("detect_one_to_one_relationships", fmt.Sprintf("%t", !params.PostgrestV9Compat))
	case LangSwift:
		values.Set("access_control", params.SwiftAccessControl)
	}
	parsed.RawQuery = values.Encode()
	body, err := access.DoRequestRaw(ctx, http.MethodGet, parsed.String(), nil, volcengine.WithTimeout(volcengine.DBRequestTimeout))
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return errors.Errorf("failed to print generated types: %w", err)
	}
	fmt.Fprintln(w)
	return nil
}
