package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ddtfch/codeaudit/internal/config"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

type AnthropicProvider struct {
	apiKey string
	model  string
}

func NewAnthropicProvider(pc config.ProviderConfig) (*AnthropicProvider, error) {
	key := os.Getenv(pc.APIKeyEnv)
	if key == "" && pc.APIKeyEnv != "" {
		return nil, fmt.Errorf("env var %s not set", pc.APIKeyEnv)
	}
	return &AnthropicProvider{apiKey: key, model: "claude-sonnet-4-6"}, nil
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) Chat(ctx context.Context, messages []Message, tools []ToolSchema) (*Response, error) {
	// Build Anthropic messages format.
	type antContent struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}
	type antMessage struct {
		Role    string     `json:"role"`
		Content []antContent `json:"content"`
	}

	var system string
	var antMsgs []antMessage
	for _, m := range messages {
		if m.Role == RoleSystem {
			system = m.Content
			continue
		}
		role := m.Role
		if role == RoleTool {
			role = RoleUser
		}
		antMsgs = append(antMsgs, antMessage{
			Role:    role,
			Content: []antContent{{Type: "text", Text: m.Content}},
		})
	}

	// Build tools array.
	type antTool struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		InputSchema map[string]any `json:"input_schema"`
	}
	var antTools []antTool
	for _, t := range tools {
		antTools = append(antTools, antTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	body := map[string]any{
		"model":      p.model,
		"max_tokens": 4096,
		"messages":   antMsgs,
	}
	if system != "" {
		body["system"] = system
	}
	if len(antTools) > 0 {
		body["tools"] = antTools
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic API error %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Input any    `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing anthropic response: %w", err)
	}

	r := &Response{
		Usage: Usage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
		},
	}
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			r.Content += c.Text
		case "tool_use":
			argBytes, _ := json.Marshal(c.Input)
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:        c.ID,
				Name:      c.Name,
				Arguments: string(argBytes),
			})
		}
	}
	return r, nil
}

// WithModel returns a copy of the provider using the given model string.
func (p *AnthropicProvider) WithModel(model string) *AnthropicProvider {
	cp := *p
	cp.model = model
	return &cp
}
