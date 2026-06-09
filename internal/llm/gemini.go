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

type GeminiProvider struct {
	apiKey string
	model  string
}

func NewGeminiProvider(pc config.ProviderConfig) (*GeminiProvider, error) {
	key := os.Getenv(pc.APIKeyEnv)
	if key == "" && pc.APIKeyEnv != "" {
		return nil, fmt.Errorf("env var %s not set", pc.APIKeyEnv)
	}
	return &GeminiProvider{apiKey: key, model: "gemini-2.0-flash"}, nil
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) Chat(ctx context.Context, messages []Message, tools []ToolSchema) (*Response, error) {
	apiURL := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		p.model, p.apiKey,
	)

	type gPart struct {
		Text string `json:"text,omitempty"`
	}
	type gContent struct {
		Role  string  `json:"role"`
		Parts []gPart `json:"parts"`
	}

	var contents []gContent
	for _, m := range messages {
		if m.Role == RoleSystem {
			// Gemini doesn't have a system role; prepend as user turn.
			contents = append(contents, gContent{
				Role:  "user",
				Parts: []gPart{{Text: "SYSTEM: " + m.Content}},
			})
			continue
		}
		role := m.Role
		if role == RoleAssistant {
			role = "model"
		} else {
			role = "user"
		}
		contents = append(contents, gContent{
			Role:  role,
			Parts: []gPart{{Text: m.Content}},
		})
	}

	body := map[string]any{"contents": contents}

	if len(tools) > 0 {
		type gFn struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Parameters  map[string]any `json:"parameters"`
		}
		var fns []gFn
		for _, t := range tools {
			fns = append(fns, gFn{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
		}
		body["tools"] = []map[string]any{{"functionDeclarations": fns}}
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini API error %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing gemini response: %w", err)
	}
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in gemini response")
	}

	r := &Response{
		Usage: Usage{
			InputTokens:  result.UsageMetadata.PromptTokenCount,
			OutputTokens: result.UsageMetadata.CandidatesTokenCount,
		},
	}
	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			argBytes, _ := json.Marshal(part.FunctionCall.Args)
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:        part.FunctionCall.Name,
				Name:      part.FunctionCall.Name,
				Arguments: string(argBytes),
			})
		} else {
			r.Content += part.Text
		}
	}
	return r, nil
}
