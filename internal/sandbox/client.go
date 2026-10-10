// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"
)

type Client struct {
	BaseURL     string
	Token       string
	AccessToken string
	HTTP        *http.Client
}

type SandboxListOptions struct {
	Metadata map[string]string
	States   []string
	Limit    int
}

type CodeResult struct {
	Stdout  string            `json:"stdout,omitempty"`
	Stderr  string            `json:"stderr,omitempty"`
	Results []json.RawMessage `json:"results,omitempty"`
	Error   json.RawMessage   `json:"error,omitempty"`
}

func (c Client) Request(ctx context.Context, method, path, sandboxID string, body []byte) (json.RawMessage, error) {
	return c.do(ctx, method, path, sandboxID, body)
}

func (c Client) ListSandboxes(ctx context.Context, options SandboxListOptions) (json.RawMessage, error) {
	query := url.Values{}
	if len(options.Metadata) > 0 {
		metadata := make([]string, 0, len(options.Metadata))
		for key, value := range options.Metadata {
			metadata = append(metadata, key+"="+value)
		}
		sort.Strings(metadata)
		query.Set("metadata", strings.Join(metadata, "&"))
	}
	if len(options.States) > 0 {
		query.Set("state", strings.Join(options.States, ","))
	}
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	path := "/sandbox/v1/v2/sandboxes"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.do(ctx, http.MethodGet, path, "", nil)
}

type CreateRequest struct {
	TemplateID          string            `json:"templateID"`
	Timeout             int               `json:"timeout,omitempty"`
	Secure              *bool             `json:"secure,omitempty"`
	AllowInternetAccess *bool             `json:"allow_internet_access,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	EnvVars             map[string]string `json:"envVars,omitempty"`
}

func (c Client) Create(ctx context.Context, request CreateRequest) (json.RawMessage, error) {
	if strings.TrimSpace(request.TemplateID) == "" {
		return nil, fmt.Errorf("template ID is required")
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode sandbox create request: %w", err)
	}
	return c.do(ctx, http.MethodPost, "/sandbox/v1/sandboxes", "", data)
}

func (c Client) StartProcess(ctx context.Context, sandboxID, command string, args []string, cwd string) (json.RawMessage, error) {
	return c.StartProcessWithOptions(ctx, sandboxID, command, args, cwd, "user", nil, false)
}

func (c Client) StartProcessWithOptions(ctx context.Context, sandboxID, command string, args []string, cwd, user string, envs map[string]string, background bool) (json.RawMessage, error) {
	if strings.TrimSpace(sandboxID) == "" {
		return nil, fmt.Errorf("sandbox ID is required")
	}
	if strings.TrimSpace(command) == "" {
		return nil, fmt.Errorf("command is required")
	}
	request := map[string]any{
		"process": map[string]any{
			"cmd":  command,
			"args": args,
			"cwd":  cwd,
			"envs": envs,
		},
		"stdin": false,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode sandbox process request: %w", err)
	}
	return c.processConnectRequest(ctx, "/sandbox/v1/process.Process/Start", sandboxID, payload, user, background)
}

func (c Client) ListFiles(ctx context.Context, sandboxID, filePath string, depth int) (json.RawMessage, error) {
	if depth < 1 {
		return nil, fmt.Errorf("depth must be at least 1")
	}
	return c.filesystemRPC(ctx, sandboxID, "ListDir", map[string]any{
		"path":  filePath,
		"depth": depth,
	})
}

func (c Client) ReadFile(ctx context.Context, sandboxID, path string) ([]byte, error) {
	return c.fileRequestBytes(ctx, http.MethodGet, sandboxID, path, nil, "application/octet-stream")
}

func (c Client) FileExists(ctx context.Context, sandboxID, path string) (bool, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return false, fmt.Errorf("sandbox URL is empty")
	}
	query := url.Values{}
	query.Set("path", path)
	query.Set("username", "user")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/sandbox/v1/files?"+query.Encode(), nil)
	if err != nil {
		return false, fmt.Errorf("create sandbox file request: %w", err)
	}
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	if strings.TrimSpace(c.Token) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
		request.Header.Set("apikey", strings.TrimSpace(c.Token))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return false, fmt.Errorf("sandbox file request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return false, fmt.Errorf("sandbox file request failed (%s): %s", response.Status, strings.TrimSpace(string(body)))
	}
	return true, nil
}

func (c Client) WriteFile(ctx context.Context, sandboxID, path string, content []byte) (json.RawMessage, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", pathpkg.Base(path))
	if err != nil {
		return nil, fmt.Errorf("create sandbox file form: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, fmt.Errorf("write sandbox file form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close sandbox file form: %w", err)
	}
	return c.fileRequestWithContentType(ctx, http.MethodPost, sandboxID, path, body.Bytes(), writer.FormDataContentType())
}

func (c Client) MakeDir(ctx context.Context, sandboxID, path string) (json.RawMessage, error) {
	return c.filesystemRPC(ctx, sandboxID, "MakeDir", map[string]string{"path": path})
}

func (c Client) FileInfo(ctx context.Context, sandboxID, path string) (json.RawMessage, error) {
	return c.filesystemRPC(ctx, sandboxID, "Stat", map[string]string{"path": path})
}

func (c Client) RemoveFile(ctx context.Context, sandboxID, path string) (json.RawMessage, error) {
	return c.filesystemRPC(ctx, sandboxID, "Remove", map[string]string{"path": path})
}

func (c Client) RenameFile(ctx context.Context, sandboxID, source, destination string) (json.RawMessage, error) {
	return c.filesystemRPC(ctx, sandboxID, "Move", map[string]string{"source": source, "destination": destination})
}

func (c Client) filesystemRPC(ctx context.Context, sandboxID, method string, request any) (json.RawMessage, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode filesystem request: %w", err)
	}
	return c.filesystemConnectRequest(ctx, "/sandbox/v1/filesystem.Filesystem/"+method, sandboxID, payload)
}

func (c Client) filesystemConnectRequest(ctx context.Context, path, sandboxID string, payload []byte) (json.RawMessage, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create filesystem Connect request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Basic "+basicAuthValue("user"))
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	request.Header.Set("E2b-Sandbox-Port", "49983")
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("filesystem Connect request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read filesystem Connect response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("filesystem Connect request failed (%s): %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.RawMessage(responseBody), nil
}

func (c Client) ExecuteCode(ctx context.Context, sandboxID, code string) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return nil, fmt.Errorf("encode sandbox code request: %w", err)
	}
	return c.doStreamJSON(ctx, http.MethodPost, "/sandbox/v1/jupyter/"+url.PathEscape(sandboxID)+"/execute", sandboxID, body)
}

func (c Client) Health(ctx context.Context, sandboxID string) (json.RawMessage, error) {
	return c.dataPlaneRequest(ctx, http.MethodGet, "/sandbox/v1/health", sandboxID, nil)
}

func (c Client) PreviewURL(sandboxID string, port int) string {
	return strings.TrimRight(c.BaseURL, "/") + "/sandbox/proxy/" + url.PathEscape(sandboxID) + "/" + fmt.Sprint(port) + "/"
}

func (c Client) connectRequest(ctx context.Context, path, sandboxID string, payload []byte) (json.RawMessage, error) {
	frame := encodeConnectFrame(payload)
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(frame))
	if err != nil {
		return nil, fmt.Errorf("create Connect request: %w", err)
	}
	request.Header.Set("Content-Type", "application/connect+json")
	request.Header.Set("Accept", "application/connect+json")
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	request.Header.Set("E2b-Sandbox-Port", "49983")
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	request.Header.Set("Authorization", "Basic "+basicAuthValue("user"))
	if strings.TrimSpace(c.Token) != "" {
		request.Header.Set("apikey", strings.TrimSpace(c.Token))
	}
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("Connect request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read Connect response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Connect request failed (%s): %s", response.Status, strings.TrimSpace(string(body)))
	}
	frames := decodeConnectFrames(body)
	if len(frames) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.RawMessage(frames[len(frames)-1]), nil
}

type processResult struct {
	Stdout   string            `json:"stdout,omitempty"`
	Stderr   string            `json:"stderr,omitempty"`
	ExitCode int               `json:"exit_code"`
	Status   string            `json:"status,omitempty"`
	Error    string            `json:"error,omitempty"`
	Events   []json.RawMessage `json:"events,omitempty"`
}

func (c Client) processConnectRequest(ctx context.Context, path, sandboxID string, payload []byte, user string, background bool) (json.RawMessage, error) {
	return c.processConnectRequestAsUser(ctx, path, sandboxID, payload, user, background)
}

func (c Client) processConnectRequestAsUser(ctx context.Context, path, sandboxID string, payload []byte, user string, background bool) (json.RawMessage, error) {
	frame := encodeConnectFrame(payload)
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(frame))
	if err != nil {
		return nil, fmt.Errorf("create process Connect request: %w", err)
	}
	request.Header.Set("Content-Type", "application/connect+json")
	request.Header.Set("Accept", "application/connect+json")
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	request.Header.Set("E2b-Sandbox-Port", "49983")
	if strings.TrimSpace(user) == "" {
		user = "user"
	}
	request.Header.Set("Authorization", "Basic "+basicAuthValue(user))
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("process Connect request failed: %w", err)
	}
	defer response.Body.Close()
	if background {
		return readBackgroundProcessResponse(response.Body)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read process Connect response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("process Connect request failed (%s): %s", response.Status, strings.TrimSpace(string(body)))
	}
	frames := decodeConnectFrames(body)
	if len(frames) == 0 {
		trimmed := bytes.TrimSpace(body)
		if len(trimmed) == 0 {
			return nil, fmt.Errorf("process Connect response was empty")
		}
		return nil, fmt.Errorf("process Connect response contained no valid frames: %s", string(trimmed))
	}
	return aggregateProcessFrames(frames)
}

func readBackgroundProcessResponse(body io.Reader) (json.RawMessage, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(body, header); err != nil {
		return nil, fmt.Errorf("read background process start frame: %w", err)
	}
	if header[0] != 0 {
		return nil, fmt.Errorf("background process start frame has unexpected flags %d", header[0])
	}
	payload := make([]byte, int(binary.BigEndian.Uint32(header[1:])))
	if _, err := io.ReadFull(body, payload); err != nil {
		return nil, fmt.Errorf("read background process start event: %w", err)
	}
	var event struct {
		Event *struct {
			Start *struct {
				PID int `json:"pid"`
			} `json:"start"`
		} `json:"event"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode background process start event: %w", err)
	}
	if event.Event == nil || event.Event.Start == nil {
		return nil, fmt.Errorf("background process response did not contain a start event: %s", string(payload))
	}
	return json.Marshal(map[string]any{
		"background": true,
		"pid":        event.Event.Start.PID,
	})
}

func aggregateProcessFrames(frames [][]byte) (json.RawMessage, error) {
	result := processResult{}
	for _, frame := range frames {
		result.Events = append(result.Events, json.RawMessage(frame))
		var event struct {
			Event *struct {
				Data *struct {
					Stdout string `json:"stdout"`
					Stderr string `json:"stderr"`
				} `json:"data"`
				End *struct {
					ExitCode int    `json:"exitCode"`
					Exited   bool   `json:"exited"`
					Status   string `json:"status"`
					Error    string `json:"error"`
				} `json:"end"`
			} `json:"event"`
		}
		if err := json.Unmarshal(frame, &event); err != nil {
			continue
		}
		var terminalError struct {
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(frame, &terminalError); err == nil && terminalError.Error != nil {
			result.Error = terminalError.Error.Code + ": " + terminalError.Error.Message
			result.Status = result.Error
			if result.ExitCode == 0 {
				result.ExitCode = 1
			}
		}
		if event.Event == nil {
			continue
		}
		if event.Event.Data != nil {
			result.Stdout += decodeProcessOutput(event.Event.Data.Stdout)
			result.Stderr += decodeProcessOutput(event.Event.Data.Stderr)
		}
		if event.Event.End != nil {
			result.ExitCode = event.Event.End.ExitCode
			result.Status = event.Event.End.Status
			if result.Status == "" {
				result.Status = event.Event.End.Error
			}
			if event.Event.End.Error != "" {
				result.Error = event.Event.End.Error
				if result.ExitCode == 0 {
					result.ExitCode = 1
				}
			}
			if result.Status == "" {
				result.Status = fmt.Sprintf("exit code %d", result.ExitCode)
			}
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode process result: %w", err)
	}
	return encoded, nil
}

func decodeProcessOutput(encoded string) string {
	if encoded == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encoded
	}
	return string(decoded)
}

func encodeConnectFrame(payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = 0
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

func decodeConnectFrames(data []byte) [][]byte {
	var frames [][]byte
	for offset := 0; offset+5 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[offset+1 : offset+5]))
		start := offset + 5
		if data[offset] != 0 && data[offset] != 2 {
			break
		}
		if length < 0 || start+length > len(data) {
			break
		}
		frames = append(frames, data[start:start+length])
		offset = start + length
	}
	return frames
}

func basicAuthValue(username string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":"))
}

func (c Client) fileRequest(ctx context.Context, method, sandboxID, filePath string, body []byte) (json.RawMessage, error) {
	data, err := c.fileRequestBytes(ctx, method, sandboxID, filePath, body, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c Client) fileRequestWithContentType(ctx context.Context, method, sandboxID, filePath string, body []byte, contentType string) (json.RawMessage, error) {
	data, err := c.fileRequestBytes(ctx, method, sandboxID, filePath, body, contentType)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (c Client) fileRequestBytes(ctx context.Context, method, sandboxID, filePath string, body []byte, contentType string) ([]byte, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("sandbox URL is empty")
	}
	query := url.Values{}
	query.Set("path", filePath)
	query.Set("username", "user")
	request, err := http.NewRequestWithContext(ctx, method, base+"/sandbox/v1/files?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sandbox file request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	if strings.TrimSpace(c.Token) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
		request.Header.Set("apikey", strings.TrimSpace(c.Token))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("sandbox file request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read sandbox file response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("sandbox file request failed (%s): %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
}

func (c Client) do(ctx context.Context, method, path, sandboxID string, body []byte) (json.RawMessage, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("sandbox URL is empty")
	}
	request, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sandbox request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.Token) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
		request.Header.Set("apikey", strings.TrimSpace(c.Token))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("sandbox request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read sandbox response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("sandbox request failed (%s): %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.RawMessage(responseBody), nil
}

func (c Client) dataPlaneRequest(ctx context.Context, method, path, sandboxID string, body []byte) (json.RawMessage, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	request, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sandbox data-plane request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("sandbox data-plane request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read sandbox data-plane response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("sandbox data-plane request failed (%s): %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	return json.RawMessage(responseBody), nil
}

func (c Client) doStreamJSON(ctx context.Context, method, path, sandboxID string, body []byte) (json.RawMessage, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	request, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create sandbox stream request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("E2b-Sandbox-Id", strings.TrimSpace(sandboxID))
	if strings.TrimSpace(c.AccessToken) != "" {
		request.Header.Set("X-Access-Token", strings.TrimSpace(c.AccessToken))
	}
	response, err := c.httpClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("sandbox stream request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		return nil, fmt.Errorf("sandbox stream request failed (%s): %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	var result CodeResult
	decoder := json.NewDecoder(response.Body)
	for {
		var message struct {
			Type   string          `json:"type"`
			Text   string          `json:"text"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode sandbox code stream: %w", err)
		}
		switch message.Type {
		case "stdout":
			result.Stdout += message.Text
		case "stderr":
			result.Stderr += message.Text
		case "result":
			result.Results = append(result.Results, message.Result)
		case "error":
			result.Error = message.Error
		}
	}
	return json.Marshal(result)
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{}
}
