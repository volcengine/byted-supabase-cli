// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

package aigateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultModel = "bytedance/doubao-seed-2.0-mini"

const defaultHTTPTimeout = 30 * time.Second

var Models = []string{
	"bytedance/doubao-seed-2.0-pro",
	"bytedance/doubao-seed-2.0-code",
	"bytedance/doubao-seed-2.0-lite",
	"bytedance/doubao-seed-2.0-mini",
}

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type ChatRequest struct {
	Model       string          `json:"model"`
	Messages    json.RawMessage `json:"messages,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type ModelsResponse struct {
	Data []Model `json:"data"`
}

func NewMessages(system, message string) (json.RawMessage, error) {
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("message must not be empty")
	}
	messages := make([]map[string]string, 0, 2)
	if strings.TrimSpace(system) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	messages = append(messages, map[string]string{"role": "user", "content": message})
	data, err := json.Marshal(messages)
	return data, err
}

func (c Client) Chat(ctx context.Context, request ChatRequest, out io.Writer) error {
	if strings.TrimSpace(request.Model) == "" {
		request.Model = DefaultModel
	}
	if len(request.Messages) == 0 {
		return fmt.Errorf("messages must not be empty")
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode chat request: %w", err)
	}
	return c.ChatRaw(ctx, body, request.Stream, out)
}

func (c Client) ChatRaw(ctx context.Context, body []byte, stream bool, out io.Writer) error {
	response, err := c.do(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return readAPIError(response)
	}
	if stream {
		return writeSSE(response.Body, out)
	}
	_, err = io.Copy(out, response.Body)
	return err
}

func (c Client) Models(ctx context.Context) (ModelsResponse, error) {
	response, err := c.do(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return ModelsResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ModelsResponse{}, readAPIError(response)
	}
	var result ModelsResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return ModelsResponse{}, fmt.Errorf("decode models response: %w", err)
	}
	return result, nil
}

func (c Client) Check(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	response, err := c.do(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, readAPIError(response)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return time.Since(start), nil
}

func (c Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("AI Gateway URL is empty")
	}
	request, err := http.NewRequestWithContext(ctx, method, base+"/v1"+path, body)
	if err != nil {
		return nil, fmt.Errorf("create AI Gateway request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(c.Token); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return c.httpClient().Do(request)
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func readAPIError(response *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	message := strings.TrimSpace(string(data))
	if message == "" {
		message = response.Status
	}
	return fmt.Errorf("AI Gateway request failed (%s): %s", response.Status, message)
}

func writeSSE(reader io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &chunk) == nil && len(chunk.Choices) > 0 {
			if _, err := io.WriteString(out, chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		} else if _, err := io.WriteString(out, payload); err != nil {
			return err
		}
	}
	return scanner.Err()
}
