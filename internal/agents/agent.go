package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ddtfch/codeaudit/internal/llm"
	"github.com/ddtfch/codeaudit/internal/observability"
	"github.com/ddtfch/codeaudit/internal/tools"
)

// Agent is the base type shared by all specialist agents.
type Agent struct {
	Name     string
	Role     string // llm.RoleTriage | llm.RoleReasoning | llm.RoleReport
	provider llm.Provider
	registry *tools.Registry
	logger   *observability.Logger
	promptFS string // path to prompts directory (may be empty → use embedded)
}

// NewAgent constructs a base Agent.
func NewAgent(name, role string, provider llm.Provider, reg *tools.Registry, logger *observability.Logger, promptsDir string) *Agent {
	return &Agent{
		Name:     name,
		Role:     role,
		provider: provider,
		registry: reg,
		logger:   logger,
		promptFS: promptsDir,
	}
}

// SystemPrompt loads the agent's system prompt from disk (promptsDir/<name>.md) or
// falls back to the embedded default.
func (a *Agent) SystemPrompt(embedded string) string {
	if a.promptFS != "" {
		path := filepath.Join(a.promptFS, strings.ToLower(a.Name)+".md")
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}
	return embedded
}

// Chat sends a single turn to the provider and logs the exchange.
func (a *Agent) Chat(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	var toolSchemas []llm.ToolSchema
	if a.registry != nil {
		toolSchemas = a.registry.Schemas()
	}

	a.logger.Debug(fmt.Sprintf("[%s] sending %d messages to %s", a.Name, len(messages), a.provider.Name()))

	resp, err := a.provider.Chat(ctx, messages, toolSchemas)
	if err != nil {
		return nil, fmt.Errorf("agent %s: %w", a.Name, err)
	}

	a.logger.Debug(fmt.Sprintf("[%s] response: %d chars, %d tool calls, usage in=%d out=%d",
		a.Name, len(resp.Content), len(resp.ToolCalls),
		resp.Usage.InputTokens, resp.Usage.OutputTokens))

	return resp, nil
}
