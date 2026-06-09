package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ddtfch/codeaudit/internal/findings"
	"github.com/ddtfch/codeaudit/internal/llm"
)

const executorSystemPrompt = `You are the Executor agent for codeaudit, a security audit tool.

You investigate a codebase for security issues by calling available tools iteratively.
Your goal is to gather evidence for the Validator agent.

Guidelines:
- Call secret_scan early to find hardcoded credentials.
- Call dep_scan to identify vulnerable dependencies.
- Use read_file with chunked reads (offset/limit) for large files — prefer 200-line chunks with overlap.
- Use grep_file to search for patterns like "exec(", "eval(", "subprocess", "os.system", "crypto", "password".
- Collect precise file:line locations for every finding.
- When you have gathered enough evidence, reply with DONE and a JSON array of raw findings.

Raw finding format:
{"title":"...", "category":"secret|dependency|vulnerability|misconfig", "severity":"red|amber|green",
 "location":"file:line", "evidence":"...", "description":"...", "remediation":"...", "refs":[]}`

// Executor runs tools and gathers evidence.
type Executor struct {
	*Agent
}

func NewExecutor(base *Agent) *Executor {
	return &Executor{Agent: base}
}

// Gather runs the agentic tool-calling loop and returns raw findings.
// The loop is driven by the orchestrator; this method performs one exchange.
func (e *Executor) Gather(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	if len(messages) == 0 || messages[0].Role != llm.RoleSystem {
		messages = append([]llm.Message{{Role: llm.RoleSystem, Content: e.SystemPrompt(executorSystemPrompt)}}, messages...)
	}
	return e.Chat(ctx, messages)
}

// ParseRawFindings extracts a JSON findings array from the executor's final response.
func ParseRawFindings(content string) ([]findings.Finding, error) {
	// Find a JSON array in the content.
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found in executor output")
	}
	raw := content[start : end+1]

	var items []struct {
		Title       string   `json:"title"`
		Category    string   `json:"category"`
		Severity    string   `json:"severity"`
		Location    string   `json:"location"`
		Evidence    string   `json:"evidence"`
		Description string   `json:"description"`
		Remediation string   `json:"remediation"`
		Refs        []string `json:"refs"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parsing findings JSON: %w", err)
	}

	fs := make([]findings.Finding, 0, len(items))
	for _, it := range items {
		cat := findings.Category(it.Category)
		sev := findings.Severity(it.Severity)
		fs = append(fs, findings.Finding{
			ID:          findings.StableID(cat, it.Location, it.Title),
			Title:       it.Title,
			Category:    cat,
			Severity:    sev,
			Confidence:  "needs_review",
			Location:    it.Location,
			Evidence:    it.Evidence,
			Description: it.Description,
			Remediation: it.Remediation,
			Refs:        it.Refs,
			Status:      findings.StatusOpen,
			DetectedBy:  "executor",
		})
	}
	return fs, nil
}
