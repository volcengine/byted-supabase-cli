package solo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	authhooks "github.com/volcengine/byted-supabase-cli/internal/auth/hooks"
)

const authRequestTimeout = 30 * time.Second

type AuthClient struct {
	baseURL        *url.URL
	serviceRoleKey string
	httpClient     *http.Client
}

func NewAuthClient(profile Profile) (*AuthClient, error) {
	if strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return nil, fmt.Errorf("missing Service Role Key for Auth admin operations")
	}
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/"))
	if err != nil || baseURL.Scheme != "http" && baseURL.Scheme != "https" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	return &AuthClient{
		baseURL:        baseURL,
		serviceRoleKey: strings.TrimSpace(profile.ServiceRoleKey),
		httpClient:     &http.Client{Timeout: authRequestTimeout},
	}, nil
}

func (c *AuthClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	body, err := c.do(ctx, http.MethodGet, "/auth/v1/config", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode Auth config response: %w", err)
	}
	return result, nil
}

func (c *AuthClient) SetConfig(ctx context.Context, patch map[string]interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Auth config patch: %w", err)
	}
	response, err := c.do(ctx, http.MethodPatch, "/auth/v1/config", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var fullConfig map[string]interface{}
	if err := json.Unmarshal(response, &fullConfig); err != nil {
		return nil, fmt.Errorf("failed to decode Auth config patch response: %w", err)
	}
	result := make(map[string]interface{}, len(patch))
	for key, value := range patch {
		if updated, ok := fullConfig[key]; ok {
			result[key] = updated
		} else {
			result[key] = value
		}
	}
	return result, nil
}

func (c *AuthClient) GetHooks(ctx context.Context) (map[string]interface{}, error) {
	body, err := c.do(ctx, http.MethodGet, "/auth/v1/config/hooks", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode Auth hooks response: %w", err)
	}
	return result, nil
}

func (c *AuthClient) SetHooks(ctx context.Context, patch map[string]interface{}) (map[string]interface{}, error) {
	current, err := c.GetHooks(ctx)
	if err != nil {
		return nil, err
	}
	if err := authhooks.ValidateHooksPatch(patch, current); err != nil {
		return nil, err
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Auth hooks patch: %w", err)
	}
	if _, err := c.do(ctx, http.MethodPatch, "/auth/v1/config/hooks", bytes.NewReader(body)); err != nil {
		return nil, err
	}
	updated, err := c.GetHooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("hooks config was updated, but reading it back failed: %w", err)
	}
	result := make(map[string]interface{}, len(patch))
	for key := range patch {
		value, ok := updated[key]
		if !ok {
			return nil, fmt.Errorf("hooks config key %q was not applied by the server; run `auth hooks get` to inspect the current config", key)
		}
		result[key] = value
	}
	return result, nil
}

type ThirdPartyProvider struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	OIDCIssuerURL string `json:"oidc_issuer_url,omitempty"`
	ResolvedAt    string `json:"resolved_at,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type AddThirdPartyParams struct {
	OIDCIssuerURL       string
	CustomJWKSKid       string
	CustomJWKSAlgorithm string
	CustomJWKSPublicKey string
}

func (c *AuthClient) ListThirdParty(ctx context.Context) ([]ThirdPartyProvider, error) {
	body, err := c.do(ctx, http.MethodGet, "/auth/v1/config/third-party-auth", nil)
	if err != nil {
		return nil, err
	}
	var providers []ThirdPartyProvider
	if err := json.Unmarshal(body, &providers); err != nil {
		return nil, fmt.Errorf("failed to decode third-party list response: %w", err)
	}
	return providers, nil
}

func (c *AuthClient) AddThirdParty(ctx context.Context, params AddThirdPartyParams) (map[string]interface{}, error) {
	var requestBody struct {
		OIDCIssuerURL   string `json:"oidc_issuer_url,omitempty"`
		CustomJWTSecret *struct {
			KID       string `json:"kid"`
			Algorithm string `json:"algorithm"`
			PublicKey string `json:"public_key"`
		} `json:"custom_jwt_secret,omitempty"`
	}
	if strings.TrimSpace(params.OIDCIssuerURL) != "" {
		requestBody.OIDCIssuerURL = strings.TrimSpace(params.OIDCIssuerURL)
	} else if params.CustomJWKSKid != "" || params.CustomJWKSAlgorithm != "" || params.CustomJWKSPublicKey != "" {
		requestBody.CustomJWTSecret = &struct {
			KID       string `json:"kid"`
			Algorithm string `json:"algorithm"`
			PublicKey string `json:"public_key"`
		}{
			KID: params.CustomJWKSKid, Algorithm: params.CustomJWKSAlgorithm,
			PublicKey: strings.ReplaceAll(params.CustomJWKSPublicKey, `\n`, "\n"),
		}
	} else {
		return nil, fmt.Errorf("either --oidc-issuer-url or --jwks-kid/--jwks-algorithm/--jwks-public-key is required")
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal add provider request: %w", err)
	}
	response, err := c.do(ctx, http.MethodPost, "/auth/v1/config/third-party-auth", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to decode add provider response: %w", err)
	}
	return result, nil
}

func (c *AuthClient) RemoveThirdParty(ctx context.Context, providerID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/auth/v1/config/third-party-auth/"+url.PathEscape(providerID), nil)
	return err
}

func (c *AuthClient) SyncThirdParty(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/auth/v1/config/third-party-auth/sync", nil)
	return err
}

func (c *AuthClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + path
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create Auth request: %w", err)
	}
	request.Header.Set("apikey", c.serviceRoleKey)
	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("X-Request-From", "byted-supabase-cli")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Auth response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Auth API returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}
