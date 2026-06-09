package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ddtfch/codeaudit/internal/findings"
	"github.com/ddtfch/codeaudit/internal/llm"
	"github.com/ddtfch/codeaudit/internal/report"
)

const reporterSystemPrompt = `You are the Reporter agent for codeaudit, a security audit tool.

You receive the validated, deduplicated findings for a codebase audit and must produce:
1. An executive summary (2-4 sentences: overall risk, top concerns, recommended first actions).
2. For each finding: a clear description and concrete remediation steps if not already present.

Return JSON with shape:
{
  "executive_summary": "...",
  "findings": [ <validated findings with enriched description/remediation> ]
}`

// Reporter synthesises findings into a structured report payload.
type Reporter struct {
	*Agent
}

func NewReporter(base *Agent) *Reporter {
	return &Reporter{Agent: base}
}

// Synthesise produces the final report payload from validated findings.
func (r *Reporter) Synthesise(ctx context.Context, validated []findings.Finding, source string) (*report.Payload, error) {
	data, _ := json.MarshalIndent(validated, "", "  ")
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: r.SystemPrompt(reporterSystemPrompt)},
		{Role: llm.RoleUser, Content: fmt.Sprintf(
			"Source: %s\n\nFindings to summarise:\n```json\n%s\n```",
			source, string(data),
		)},
	}

	resp, err := r.Chat(ctx, messages)
	if err != nil {
		// Fallback: use findings as-is with a generic summary.
		return fallbackPayload(validated, source), nil
	}

	return parseReportPayload(resp.Content, validated, source)
}

func parseReportPayload(content string, original []findings.Finding, source string) (*report.Payload, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return fallbackPayload(original, source), nil
	}

	var raw struct {
		ExecutiveSummary string           `json:"executive_summary"`
		Findings         []findings.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &raw); err != nil {
		return fallbackPayload(original, source), nil
	}

	fs := raw.Findings
	if len(fs) == 0 {
		fs = original
	}

	return &report.Payload{
		Source:           source,
		ExecutiveSummary: raw.ExecutiveSummary,
		Findings:         fs,
	}, nil
}

func fallbackPayload(fs []findings.Finding, source string) *report.Payload {
	return &report.Payload{
		Source:           source,
		ExecutiveSummary: fmt.Sprintf("Audit of %s: %d finding(s) identified.", source, len(fs)),
		Findings:         fs,
	}
}
