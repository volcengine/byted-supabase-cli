package solo

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type PagesClient struct {
	baseURL *url.URL
	key     string
	client  *http.Client
}

func NewPagesClient(profile Profile) (*PagesClient, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/"))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	if strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return nil, &MissingCredentialError{Name: "Service Role Key"}
	}
	return &PagesClient{baseURL: u, key: profile.ServiceRoleKey, client: http.DefaultClient}, nil
}

func LoadPagesClient() (*PagesClient, error) {
	profile, err := LoadCurrentProfile()
	if err != nil {
		return nil, err
	}
	return NewPagesClient(profile)
}

func (c *PagesClient) request(ctx context.Context, method, route string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	u := *c.baseURL
	routeURL, err := url.Parse(route)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/pages/v1" + routeURL.Path
	u.RawQuery = routeURL.RawQuery
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("X-Request-From", "byted-supabase-cli")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Pages API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (c *PagesClient) List(ctx context.Context, name string, limit, offset int) (any, error) {
	// The data-plane endpoint is branch-scoped by the URL/key pair. Never add
	// an account-wide `all=true` query or expose an account-level listing path:
	// a service_role for one Supabase instance must not enumerate other Pages
	// projects. The Pages gateway accepts the list filter as the action-shaped
	// JSON body, including pagination.
	payload := map[string]any{}
	if name != "" {
		payload["Name"] = name
	}
	if limit > 0 {
		payload["PageSize"] = limit
		payload["PageNumber"] = pageNumber(limit, offset)
	}
	var body any
	if len(payload) > 0 {
		body = payload
	}
	data, err := c.request(ctx, http.MethodGet, "/projects", body)
	return rawJSON(data, err)
}

func (c *PagesClient) Binding(ctx context.Context) (any, error) {
	data, err := c.request(ctx, http.MethodGet, "/binding", nil)
	return rawJSON(data, err)
}

func (c *PagesClient) Bind(ctx context.Context, projectID, customPrefix, frameworkPrefix string) error {
	_, err := c.request(ctx, http.MethodPut, "/binding", map[string]string{
		"PagesProjectId": projectID, "CustomPrefix": customPrefix, "FrameworkPrefix": frameworkPrefix,
	})
	return err
}

func (c *PagesClient) Unbind(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodDelete, "/binding", nil)
	return err
}

func (c *PagesClient) Sync(ctx context.Context, projectID string) error {
	_, err := c.request(ctx, http.MethodPost, "/binding/env-vars/sync", map[string]string{"PagesProjectId": projectID})
	return err
}

func (c *PagesClient) Create(ctx context.Context, params map[string]any) (any, error) {
	data, err := c.request(ctx, http.MethodPost, "/projects", params)
	return rawJSON(data, err)
}

func (c *PagesClient) Deploy(ctx context.Context, projectID, resourceID string) (any, error) {
	// Deployment is fire-and-return. The central Pages component is rate-limited;
	// callers must use `pages deploy list` explicitly when they need status.
	data, err := c.request(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/deploys", map[string]string{
		"PagesProjectId": projectID, "ProjectDeployResourceId": resourceID,
	})
	return rawJSON(data, err)
}

func (c *PagesClient) DeployList(ctx context.Context, projectID string, limit, offset int) (any, error) {
	data, err := c.request(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectID)+"/deploys", map[string]any{
		"PagesProjectId": projectID, "PageSize": limit, "PageNumber": pageNumber(limit, offset),
	})
	return rawJSON(data, err)
}

func (c *PagesClient) Upload(ctx context.Context, filename string) (any, error) {
	data, err := c.request(ctx, http.MethodPost, "/resources", map[string]string{"FileName": filename})
	if err != nil {
		return nil, err
	}
	var result struct {
		ResourceID string `json:"ProjectDeployResourceId"`
		UploadURL  string `json:"TosUploadUrl"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, result.UploadURL, file)
	if err != nil {
		return nil, err
	}
	req.ContentLength = stat.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Pages resource upload returned HTTP %d", resp.StatusCode)
	}
	return map[string]any{"file": filepath.Base(filename), "project_deploy_resource_id": result.ResourceID}, nil
}

func rawJSON(data []byte, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	if encoded, ok := value.(string); ok {
		decoded, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			decoded, decodeErr = base64.RawStdEncoding.DecodeString(encoded)
		}
		if decodeErr == nil {
			var decodedValue any
			if json.Unmarshal(decoded, &decodedValue) == nil {
				return decodedValue, nil
			}
		}
	}
	return value, nil
}

func pageNumber(limit, offset int) int {
	if limit <= 0 {
		return 0
	}
	return offset/limit + 1
}
