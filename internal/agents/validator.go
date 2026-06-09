package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ddtfch/codeaudit/internal/findings"
	"github.com/ddtfch/codeaudit/internal/llm"
)

const validatorSystemPrompt = `You are the Validator agent for codeaudit, a security audit tool.

You receive a list of candidate findings and must:
1. Assess each finding for false positives (test files, example code, placeholder values, already-rotated keys).
2. Set confidence: "confirmed" (you are certain it is a real issue), "likely" (strong indicator but not certain), "needs_review" (ambiguous).
3. Upgrade or downgrade severity based on exploitability context.
4. Remove obvious false positives entirely (set "drop": true).

Return a JSON array with the same fields plus "confidence" and optional "drop": true.
Be strict: a hallucinated critical finding is worse than a missed low-severity one.
For confirmed-high findings, briefly explain WHY you are confident.`

// Validator reduces false positives and sets confidence levels.
type Validator struct {
	*Agent
}

func NewValidator(base *Agent) *Validator {
	return &Validator{Agent: base}
}

// Validate takes candidate findings and returns a cleaned, confidence-scored list.
func (v *Validator) Validate(ctx context.Context, candidates []findings.Finding) ([]findings.Finding, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	data, _ := json.MarshalIndent(candidates, "", "  ")
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: v.SystemPrompt(validatorSystemPrompt)},
		{Role: llm.RoleUser, Content: fmt.Sprintf("Validate these findings:\n```json\n%s\n```", string(data))},
	}

	resp, err := v.Chat(ctx, messages)
	if err != nil {
		return candidates, err // on error, return unvalidated
	}

	return parseValidatedFindings(resp.Content, candidates)
}

func parseValidatedFindings(content string, original []findings.Finding) ([]findings.Finding, error) {
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start < 0 || end <= start {
		// Could not parse — return originals unchanged.
		return original, nil
	}

	var items []struct {
		findings.Finding
		Drop bool `json:"drop"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &items); err != nil {
		return original, nil
	}

	var result []findings.Finding
	for _, item := range items {
		if item.Drop {
			continue
		}
		f := item.Finding
		// Re-compute RAG severity from confidence + raw severity.
		f.Severity = findings.MapSeverity(string(f.Severity), f.Confidence)
		result = append(result, f)
	}
	return result, nil
}
