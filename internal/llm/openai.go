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

const openAIAPIURL = "https://api.openai.com/v1/chat/completions"

type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
}

func NewOpenAIProvider(pc config.ProviderConfig) (*OpenAIProvider, error) {
	key := os.Getenv(pc.APIKeyEnv)
	if key == "" && pc.APIKeyEnv != "" {
		return nil, fmt.Errorf("env var %s not set", pc.APIKeyEnv)
	}
	base := pc.BaseURL
	if base == "" {
		base = openAIAPIURL
	}
	return &OpenAIProvider{apiKey: key, baseURL: base, model: "gpt-4o"}, nil
}

func NewOpenAICompatibleProvider(pc config.ProviderConfig) (*OpenAIProvider, error) {
	key := os.Getenv(pc.APIKeyEnv)
	base := pc.BaseURL
	if base == "" {
		base = "http://localhost:11434/v1/chat/completions"
	} else {
		base = base + "/chat/completions"
	}
	return &OpenAIProvider{apiKey: key, baseURL: base, model: "qwen2.5-coder:14b"}, nil
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Chat(ctx context.Context, messages []Message, tools []ToolSchema) (*Response, error) {
	type oaiFn struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	type oaiTC struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function oaiFn  `json:"function"`
	}
	type oaiMessage struct {
		Role       string   `json:"role"`
		Content    string   `json:"content,omitempty"`
		ToolCallID string   `json:"tool_call_id,omitempty"`
		ToolCalls  []oaiTC  `json:"tool_calls,omitempty"`
	}

	var oaiMsgs []oaiMessage
	for _, m := range messages {
		om := oaiMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			om.ToolCalls = append(om.ToolCalls, oaiTC{
				ID:   tc.ID,
				Type: "function",
				Function: oaiFn{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		oaiMsgs = append(oaiMsgs, om)
	}

	type oaiToolFn struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	}
	type oaiTool struct {
		Type     string    `json:"type"`
		Function oaiToolFn `json:"function"`
	}
	var oaiTools []oaiTool
	for _, t := range tools {
		oaiTools = append(oaiTools, oaiTool{
			Type: "function",
			Function: oaiToolFn{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	body := map[string]any{
		"model":    p.model,
		"messages": oaiMsgs,
	}
	if len(oaiTools) > 0 {
		body["tools"] = oaiTools
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing openai response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in openai response")
	}

	r := &Response{
		Content: result.Choices[0].Message.Content,
		Usage: Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
		},
	}
	for _, tc := range result.Choices[0].Message.ToolCalls {
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return r, nil
}
