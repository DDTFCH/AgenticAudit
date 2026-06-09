package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ddtfch/codeaudit/internal/agents"
	"github.com/ddtfch/codeaudit/internal/findings"
	"github.com/ddtfch/codeaudit/internal/llm"
	"github.com/ddtfch/codeaudit/internal/observability"
	"github.com/ddtfch/codeaudit/internal/tools"
)

const (
	maxLoopIterations = 30
	doneSentinel      = "DONE"
)

// Loop runs the agentic tool-calling loop for the executor agent.
// It sends messages to the LLM, dispatches any tool calls, and feeds results
// back until the LLM signals it is done or iteration limits are hit.
func Loop(
	ctx context.Context,
	executor *agents.Executor,
	registry *tools.Registry,
	budget *Budget,
	logger *observability.Logger,
	initialPrompt string,
) ([]findings.Finding, error) {

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: initialPrompt},
	}

	var allFindings []findings.Finding

	for i := 0; i < maxLoopIterations; i++ {
		if err := budget.Check(); err != nil {
			logger.Warn(fmt.Sprintf("budget exceeded at iteration %d: %v", i, err))
			break
		}

		resp, err := executor.Gather(ctx, messages)
		if err != nil {
			return allFindings, fmt.Errorf("executor loop iteration %d: %w", i, err)
		}
		budget.RecordTokens(resp.Usage.InputTokens, resp.Usage.OutputTokens)

		logger.Debug(fmt.Sprintf("loop iter %d: content=%q tool_calls=%d", i, truncate(resp.Content, 100), len(resp.ToolCalls)))

		// Append assistant turn.
		assistantMsg := llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls}
		messages = append(messages, assistantMsg)

		// If the LLM signals done, try to parse findings from its content.
		if strings.Contains(resp.Content, doneSentinel) {
			if fs, err := agents.ParseRawFindings(resp.Content); err == nil {
				allFindings = append(allFindings, fs...)
			}
			break
		}

		// If no tool calls and no DONE, prompt to continue or finish.
		if len(resp.ToolCalls) == 0 {
			if resp.Content != "" {
				// Try parsing findings anyway (model may have omitted DONE).
				if fs, err := agents.ParseRawFindings(resp.Content); err == nil && len(fs) > 0 {
					allFindings = append(allFindings, fs...)
					break
				}
			}
			messages = append(messages, llm.Message{
				Role:    llm.RoleUser,
				Content: "Continue the audit. If you have gathered sufficient evidence, reply with DONE followed by the findings JSON array.",
			})
			continue
		}

		// Dispatch all tool calls and collect results.
		for _, tc := range resp.ToolCalls {
			logger.Info(fmt.Sprintf("  tool call: %s(%s)", tc.Name, truncate(tc.Arguments, 80)))

			result, toolErr := registry.Dispatch(ctx, tc)
			if toolErr != nil {
				result = fmt.Sprintf("ERROR: %v", toolErr)
			}

			// Count files if this was a list_files call.
			if tc.Name == "list_files" {
				lines := strings.Count(result, "\n") + 1
				budget.RecordFiles(lines)
			}

			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})

			// Log tool output as a trace artifact.
			logger.Trace(observability.Artifact{
				ToolName: tc.Name,
				Input:    tc.Arguments,
				Output:   truncate(result, 500),
			})
		}
	}

	return findings.Deduplicate(allFindings), nil
}

// dispatchAndAppend is a helper for callers that need single-shot tool dispatch.
func dispatchAndAppend(
	ctx context.Context,
	registry *tools.Registry,
	tc llm.ToolCall,
	messages *[]llm.Message,
) error {
	result, err := registry.Dispatch(ctx, tc)
	if err != nil {
		result = fmt.Sprintf("ERROR: %v", err)
	}
	*messages = append(*messages, llm.Message{
		Role:       llm.RoleTool,
		Content:    result,
		ToolCallID: tc.ID,
	})
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// toolResultContent builds a tool-result message compatible with Anthropic's format.
func toolResultContent(id, content string) json.RawMessage {
	m := map[string]any{
		"type":        "tool_result",
		"tool_use_id": id,
		"content":     content,
	}
	b, _ := json.Marshal(m)
	return b
}
