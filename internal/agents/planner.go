package agents

import (
	"context"
	"fmt"

	"github.com/ddtfch/codeaudit/internal/llm"
)

const plannerSystemPrompt = `You are the Planner agent for codeaudit, a security audit tool.

Your job is to scope the audit given information about the repository:
1. Call list_files to understand the repository structure.
2. Identify the most security-relevant files and directories.
3. Decide which tools to invoke: secret_scan, dep_scan, sast_scan, read_file, grep_file.
4. Return a structured audit plan as JSON with the shape:
   {"focus_paths": [...], "tools": [...], "rationale": "..."}

Be thorough but prioritise: config files, CI/CD, auth, crypto, HTTP handlers, dependency manifests.
Do not skip .env files, docker-compose files, or cloud configuration.`

// Planner produces an audit scope and execution plan.
type Planner struct {
	*Agent
}

func NewPlanner(base *Agent) *Planner {
	return &Planner{Agent: base}
}

// Plan analyses the repository structure and returns a prioritised audit plan.
func (p *Planner) Plan(ctx context.Context, repoSummary string) (string, error) {
	system := p.SystemPrompt(plannerSystemPrompt)
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: fmt.Sprintf("Repository to audit:\n%s\n\nPlease scope this audit.", repoSummary)},
	}

	resp, err := p.Chat(ctx, messages)
	if err != nil {
		return "", err
	}
	if len(resp.ToolCalls) > 0 {
		// Planner may call list_files; handle via the executor loop in orchestrator.
		return fmt.Sprintf("tool_calls:%d content:%s", len(resp.ToolCalls), resp.Content), nil
	}
	return resp.Content, nil
}
