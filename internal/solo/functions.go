package solo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"github.com/volcengine/byted-supabase-cli/internal/functions/deploy"
	functionsnew "github.com/volcengine/byted-supabase-cli/internal/functions/new"
	"github.com/volcengine/byted-supabase-cli/internal/utils"
	"github.com/volcengine/byted-supabase-cli/internal/utils/flags"
	"github.com/volcengine/byted-supabase-cli/pkg/api"
)

type FunctionsClient struct {
	baseURL        *url.URL
	serviceRoleKey string
	httpClient     *http.Client
}

const functionsRequestTimeout = 30 * time.Second

type FunctionBody struct {
	Version int            `json:"version"`
	Files   []FunctionFile `json:"files"`
}

type FunctionFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type DeployRequest struct {
	Slug     string         `json:"slug"`
	Metadata DeployMetadata `json:"metadata"`
	Files    []FunctionFile `json:"files"`
}

type DeployMetadata struct {
	EntrypointPath string `json:"entrypoint_path"`
	Name           string `json:"name"`
	Runtime        string `json:"runtime"`
	VerifyJWT      *bool  `json:"verify_jwt,omitempty"`
	ImportMapPath  string `json:"import_map_path,omitempty"`
}

func NewFunctionsClient(profile Profile) (*FunctionsClient, error) {
	if strings.TrimSpace(profile.ServiceRoleKey) == "" {
		return nil, &MissingCredentialError{Name: "Service Role Key"}
	}
	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(profile.SupabaseURL), "/"))
	if err != nil || baseURL.Scheme != "http" && baseURL.Scheme != "https" || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid Supabase URL %q", profile.SupabaseURL)
	}
	return &FunctionsClient{
		baseURL:        baseURL,
		serviceRoleKey: strings.TrimSpace(profile.ServiceRoleKey),
		httpClient:     &http.Client{Timeout: functionsRequestTimeout},
	}, nil
}

func LoadFunctionsClient() (*FunctionsClient, error) {
	profile, err := LoadCurrentProfile()
	if err != nil {
		return nil, err
	}
	return NewFunctionsClient(profile)
}

func (c *FunctionsClient) List(ctx context.Context) ([]api.FunctionResponse, error) {
	body, err := c.do(ctx, http.MethodGet, "/functions/v1/", nil)
	if err != nil {
		return nil, err
	}
	var functions []struct {
		CreatedAt      string                     `json:"created_at"`
		EntrypointPath *string                    `json:"entrypoint_path,omitempty"`
		EzbrSHA256     *string                    `json:"ezbr_sha256,omitempty"`
		ID             string                     `json:"id"`
		ImportMap      *bool                      `json:"import_map,omitempty"`
		ImportMapPath  *string                    `json:"import_map_path,omitempty"`
		Name           string                     `json:"name"`
		Slug           string                     `json:"slug"`
		Status         api.FunctionResponseStatus `json:"status"`
		UpdatedAt      string                     `json:"updated_at"`
		VerifyJWT      *bool                      `json:"verify_jwt,omitempty"`
		Version        int                        `json:"version"`
	}
	if err := json.Unmarshal(body, &functions); err != nil {
		return nil, errors.Errorf("failed to decode functions list response: %w", err)
	}
	result := make([]api.FunctionResponse, 0, len(functions))
	for _, function := range functions {
		createdAt, err := parseFunctionTime(function.CreatedAt)
		if err != nil {
			return nil, errors.Errorf("failed to decode function %s created_at: %w", function.Slug, err)
		}
		updatedAt, err := parseFunctionTime(function.UpdatedAt)
		if err != nil {
			return nil, errors.Errorf("failed to decode function %s updated_at: %w", function.Slug, err)
		}
		result = append(result, api.FunctionResponse{
			CreatedAt: createdAt, EntrypointPath: function.EntrypointPath,
			EzbrSha256: function.EzbrSHA256, Id: function.ID, ImportMap: function.ImportMap,
			ImportMapPath: function.ImportMapPath, Name: function.Name, Slug: function.Slug,
			Status: function.Status, UpdatedAt: updatedAt, VerifyJwt: function.VerifyJWT,
			Version: function.Version,
		})
	}
	return result, nil
}

func (c *FunctionsClient) GetBody(ctx context.Context, slug string) (FunctionBody, error) {
	body, err := c.do(ctx, http.MethodGet, "/functions/v1/"+url.PathEscape(slug)+"/body", nil)
	if err != nil {
		return FunctionBody{}, err
	}
	var result FunctionBody
	if err := json.Unmarshal(body, &result); err != nil {
		return FunctionBody{}, errors.Errorf("failed to decode function body response: %w", err)
	}
	return result, nil
}

func (c *FunctionsClient) Delete(ctx context.Context, slug string) error {
	_, err := c.do(ctx, http.MethodDelete, "/functions/v1/"+url.PathEscape(slug), nil)
	return err
}

func (c *FunctionsClient) Deploy(ctx context.Context, request DeployRequest) ([]byte, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Errorf("failed to encode deploy request: %w", err)
	}
	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + "/functions/v1/deploy"
	query := requestURL.Query()
	query.Set("slug", request.Slug)
	requestURL.RawQuery = query.Encode()
	return c.doURL(ctx, http.MethodPost, requestURL.String(), body)
}

func (c *FunctionsClient) DeployLocal(ctx context.Context, slug, runtime string, noVerifyJWT *bool, fsys afero.Fs) error {
	if err := utils.ValidateFunctionSlug(slug); err != nil {
		return err
	}
	if err := flags.LoadConfig(fsys); err != nil {
		return err
	}
	runtimeValue, err := deploy.NormalizeVolcengineRuntime(slug, runtime, fsys)
	if err != nil {
		return err
	}
	verifyJWT := true
	if noVerifyJWT != nil {
		verifyJWT = !*noVerifyJWT
	}
	functionDir := filepath.Join(utils.FunctionsDir, slug)
	files, err := collectFunctionFiles(fsys, functionDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", functionDir)
	}
	if strings.HasPrefix(strings.TrimSpace(runtimeValue), "native-python") && !hasFunctionFile(files, "app.py") {
		return fmt.Errorf("Python runtime requires app.py in %s", functionDir)
	}
	files = ensurePythonScaffold(files, runtimeValue)
	_, err = c.Deploy(ctx, DeployRequest{
		Slug: slug,
		Metadata: DeployMetadata{
			EntrypointPath: deploy.DefaultVolcengineEntrypoint(runtimeValue),
			Name:           slug,
			Runtime:        runtimeValue,
			VerifyJWT:      &verifyJWT,
		},
		Files: files,
	})
	return err
}

func ensurePythonScaffold(files []FunctionFile, runtime string) []FunctionFile {
	if !strings.HasPrefix(strings.TrimSpace(runtime), "native-python") {
		return files
	}
	if !hasFunctionFile(files, "run.sh") {
		files = append(files, FunctionFile{Name: "run.sh", Content: functionsnew.PythonRunSh()})
	}
	if !hasFunctionFile(files, "requirements.txt") {
		files = append(files, FunctionFile{Name: "requirements.txt", Content: functionsnew.PythonRequirements()})
	}
	return files
}

func hasFunctionFile(files []FunctionFile, name string) bool {
	for _, file := range files {
		if file.Name == name {
			return true
		}
	}
	return false
}

func collectFunctionFiles(fsys afero.Fs, functionDir string) ([]FunctionFile, error) {
	files := make([]FunctionFile, 0)
	err := afero.Walk(fsys, functionDir, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(functionDir, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if name == "" || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return fmt.Errorf("invalid function file path %q", name)
		}
		content, err := afero.ReadFile(fsys, filePath)
		if err != nil {
			return err
		}
		files = append(files, FunctionFile{Name: name, Content: string(content)})
		return nil
	})
	if err != nil {
		return nil, errors.Errorf("failed to collect function files: %w", err)
	}
	return files, nil
}

func (c *FunctionsClient) Download(ctx context.Context, slug, root string, fsys afero.Fs) error {
	body, err := c.GetBody(ctx, slug)
	if err != nil {
		return err
	}
	for _, file := range body.Files {
		destination, err := functionFilePath(root, slug, file.Name)
		if err != nil {
			return err
		}
		if err := fsys.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return errors.Errorf("failed to create function directory: %w", err)
		}
		if err := utilsWriteFile(fsys, destination, []byte(file.Content)); err != nil {
			return errors.Errorf("failed to save function file: %w", err)
		}
	}
	return nil
}

func functionFilePath(root, slug, name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	prefix := path.Join(root, slug) + "/"
	name = strings.TrimPrefix(name, prefix)
	name = strings.TrimPrefix(name, strings.Trim(slug, "/")+"/")
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid function file path %q", name)
	}
	return filepath.Join(root, slug, filepath.FromSlash(clean)), nil
}

func utilsWriteFile(fsys afero.Fs, filename string, data []byte) error {
	if err := afero.WriteFile(fsys, filename, data, 0644); err != nil {
		return err
	}
	return nil
}

func parseFunctionTime(value string) (int64, error) {
	timestamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0, err
	}
	return timestamp.UnixMilli(), nil
}

func (c *FunctionsClient) do(ctx context.Context, method, requestPath string, body []byte) ([]byte, error) {
	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + requestPath
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(string(body))
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("apikey", c.serviceRoleKey)
	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("X-Request-From", "byted-supabase-cli")
	return c.doRequest(ctx, request)
}

func (c *FunctionsClient) doURL(ctx context.Context, method, requestURL string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apikey", c.serviceRoleKey)
	request.Header.Set("Authorization", "Bearer "+c.serviceRoleKey)
	request.Header.Set("X-Request-From", "byted-supabase-cli")
	return c.doRequest(ctx, request)
}

func (c *FunctionsClient) doRequest(_ context.Context, request *http.Request) ([]byte, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Functions API returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}
