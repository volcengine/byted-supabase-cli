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
	"time"

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

type SupabaseParams struct {
	SupabaseURL        string
	ServiceRoleKey     string
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

func RunSupabase(ctx context.Context, params SupabaseParams, w io.Writer) error {
	schemas := params.Schemas
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}
	base := strings.TrimRight(strings.TrimSpace(params.SupabaseURL), "/")
	if base == "" {
		return errors.New("missing Supabase URL")
	}
	if strings.TrimSpace(params.ServiceRoleKey) == "" {
		return errors.New("missing service role key; types generation requires service_role authentication")
	}
	query := url.Values{}
	query.Set("included_schemas", strings.Join(schemas, ","))
	if params.Lang == LangTypescript {
		query.Set("detect_one_to_one_relationships", fmt.Sprintf("%t", !params.PostgrestV9Compat))
	}
	if params.Lang == LangSwift {
		query.Set("access_control", params.SwiftAccessControl)
	}
	endpoint := base + "/postgres/generators/" + url.PathEscape(params.Lang) + "?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("apikey", params.ServiceRoleKey)
	request.Header.Set("Authorization", "Bearer "+params.ServiceRoleKey)
	response, err := (&http.Client{Timeout: time.Minute}).Do(request)
	if err != nil {
		return errors.Errorf("failed to generate types: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.Errorf("types generation failed (%s): %s", response.Status, strings.TrimSpace(string(body)))
	}
	_, err = w.Write(append(body, '\n'))
	return err
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
