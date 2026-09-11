package solo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/go-errors/errors"
	"github.com/joho/godotenv"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type SecretsClient struct {
	baseURL        *url.URL
	serviceRoleKey string
	httpClient     *http.Client
}

func NewSecretsClient(profile Profile) (*SecretsClient, error) {
	if strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return nil, &MissingCredentialError{Name: "Service Role Key"}
	}
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/"))
	if err != nil || !isSecureSecretsURL(baseURL) {
		return nil, fmt.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	return &SecretsClient{
		baseURL:        baseURL,
		serviceRoleKey: strings.TrimSpace(profile.ServiceRoleKey),
		httpClient:     http.DefaultClient,
	}, nil
}

func isSecureSecretsURL(baseURL *url.URL) bool {
	if baseURL == nil || baseURL.Host == "" {
		return false
	}
	if baseURL.Scheme == "https" {
		return true
	}
	if baseURL.Scheme != "http" {
		return false
	}
	switch strings.ToLower(baseURL.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func LoadSecretsClient() (*SecretsClient, error) {
	profile, err := LoadCurrentProfile()
	if err != nil {
		return nil, err
	}
	return NewSecretsClient(profile)
}

func (c *SecretsClient) List(ctx context.Context) ([]api.SecretResponse, error) {
	body, err := c.do(ctx, http.MethodGet, "/v1/projects/default/secrets", nil)
	if err != nil {
		return nil, err
	}
	var secrets []api.SecretResponse
	if err := json.Unmarshal(body, &secrets); err != nil {
		return nil, errors.Errorf("failed to decode secrets list response: %w", err)
	}
	sort.Slice(secrets, func(i, j int) bool {
		return secrets[i].Name < secrets[j].Name
	})
	return secrets, nil
}

func (c *SecretsClient) Set(ctx context.Context, secrets api.CreateSecretBody) error {
	body, err := json.Marshal(secrets)
	if err != nil {
		return errors.Errorf("failed to encode secrets request: %w", err)
	}
	_, err = c.do(ctx, http.MethodPost, "/v1/projects/default/secrets", body)
	return err
}

func (c *SecretsClient) Unset(ctx context.Context, names []string) error {
	body, err := json.Marshal(names)
	if err != nil {
		return errors.Errorf("failed to encode secrets unset request: %w", err)
	}
	_, err = c.do(ctx, http.MethodDelete, "/v1/projects/default/secrets", body)
	return err
}

func ParseSecrets(envFile string, fsys afero.Fs, args ...string) (api.CreateSecretBody, error) {
	values := map[string]string{}
	if strings.TrimSpace(envFile) != "" {
		file, err := fsys.Open(envFile)
		if err != nil {
			return nil, errors.Errorf("failed to open env file: %w", err)
		}
		parsed, parseErr := godotenv.Parse(file)
		_ = file.Close()
		if parseErr != nil {
			return nil, errors.Errorf("failed to parse env file: %w", parseErr)
		}
		for name, value := range parsed {
			values[name] = value
		}
	}
	for _, pair := range args {
		name, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("Invalid secret pair: %s. Must be NAME=VALUE.", pair)
		}
		values[name] = value
	}
	result := make(api.CreateSecretBody, 0, len(values))
	for name, value := range values {
		if strings.HasPrefix(name, "SUPABASE_") {
			continue
		}
		result = append(result, api.CreateSecretBody{{Name: name, Value: value}}...)
	}
	return result, nil
}

func (c *SecretsClient) do(ctx context.Context, method, requestPath string, body []byte) ([]byte, error) {
	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + requestPath
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
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
	responseBody := make([]byte, 0)
	responseBody, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Secrets API returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}
