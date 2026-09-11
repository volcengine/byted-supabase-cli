package solo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const endpointsRequestTimeout = 30 * time.Second

type Endpoint struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type EndpointCheck struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status int    `json:"status"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

type EndpointsClient struct {
	baseURL        *url.URL
	serviceRoleKey string
	httpClient     *http.Client
}

func NewEndpointsClient(profile Profile) (*EndpointsClient, error) {
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/"))
	if err != nil || baseURL.Scheme != "http" && baseURL.Scheme != "https" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	key := strings.TrimSpace(profile.ServiceRoleKey)
	if key == "" {
		key = strings.TrimSpace(profile.AnonKey)
	}
	if key == "" {
		return nil, &MissingCredentialError{Name: "Anon Key or Service Role Key"}
	}
	return &EndpointsClient{
		baseURL:        baseURL,
		serviceRoleKey: key,
		httpClient:     &http.Client{Timeout: endpointsRequestTimeout},
	}, nil
}

func LoadEndpointsClient() (*EndpointsClient, error) {
	profile, err := LoadCurrentProfile()
	if err != nil {
		return nil, err
	}
	return NewEndpointsClient(profile)
}

func (c *EndpointsClient) List() []Endpoint {
	base := strings.TrimRight(c.baseURL.String(), "/")
	return []Endpoint{
		{Name: "auth", URL: base + "/auth/v1"},
		{Name: "rest", URL: base + "/rest/v1"},
		{Name: "storage", URL: base + "/storage/v1"},
		{Name: "functions", URL: base + "/functions/v1"},
	}
}

func (c *EndpointsClient) Check(ctx context.Context) []EndpointCheck {
	results := make([]EndpointCheck, 0, len(c.List()))
	for _, endpoint := range c.List() {
		result := EndpointCheck{Name: endpoint.Name, URL: endpoint.URL}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		request.Header.Set("apikey", c.serviceRoleKey)
		request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
		request.Header.Set("X-Request-From", "byted-supabase-cli")
		response, err := c.httpClient.Do(request)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		response.Body.Close()
		result.Status = response.StatusCode
		result.OK = response.StatusCode >= 200 && response.StatusCode < 500
		results = append(results, result)
	}
	return results
}
