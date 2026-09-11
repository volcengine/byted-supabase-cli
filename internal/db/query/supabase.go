package query

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-errors/errors"
	"github.com/volcengine/byted-supabase-cli/internal/volcengine"
)

type SupabaseHTTPParams struct {
	SupabaseURL    string
	ServiceRoleKey string
	SQL            string
	Format         string
	AgentMode      bool
}

type supabaseHTTPQueryRequest struct {
	Query string `json:"query"`
}

func RunSupabaseHTTP(ctx context.Context, params SupabaseHTTPParams, w io.Writer) error {
	body, err := ExecuteSupabaseHTTP(ctx, params)
	if err != nil {
		return err
	}
	return formatLinkedResponse(w, body, params.Format, params.AgentMode)
}

func ExecuteSupabaseHTTP(ctx context.Context, params SupabaseHTTPParams) ([]byte, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(params.SupabaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("missing Supabase URL")
	}
	key := strings.TrimSpace(params.ServiceRoleKey)
	if key == "" {
		return nil, errors.New("missing service role key; /postgres/query requires service_role authentication")
	}
	if strings.TrimSpace(params.SQL) == "" {
		return nil, errors.New("missing SQL query")
	}

	requestBody, err := json.Marshal(supabaseHTTPQueryRequest{
		Query: params.SQL,
	})
	if err != nil {
		return nil, errors.Errorf("failed to encode query request: %w", err)
	}
	queryURL := baseURL + "/postgres/query"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, errors.Errorf("failed to create query request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apikey", key)
	request.Header.Set("Authorization", "Bearer "+key)

	response, err := (&http.Client{Timeout: volcengine.DBRequestTimeout}).Do(request)
	if err != nil {
		return nil, errors.Errorf("failed to execute query: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, errors.Errorf("failed to read query response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = response.Status
		}
		return nil, fmt.Errorf("query request failed (%s): %s", response.Status, message)
	}
	return responseBody, nil
}
