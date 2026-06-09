package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ddtfch/codeaudit/internal/llm"
)

// RegisterSAST registers the sast_scan tool (requires semgrep in PATH).
func RegisterSAST(r *Registry, root string) {
	r.Register(llm.ToolSchema{
		Name:        "sast_scan",
		Description: "Run semgrep static analysis on the repository. Returns SAST findings with rule IDs and locations.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ruleset": map[string]any{
					"type":        "string",
					"description": "Semgrep ruleset, e.g. 'auto' or 'p/owasp-top-ten'. Default: auto.",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var a struct {
			Ruleset string `json:"ruleset"`
		}
		a.Ruleset = "auto"
		json.Unmarshal(args, &a) //nolint:errcheck

		return runSemgrep(ctx, root, a.Ruleset)
	})
}

func runSemgrep(ctx context.Context, root, ruleset string) (string, error) {
	result, err := runSandboxed(ctx, root, "semgrep", []string{
		"--config", ruleset,
		"--json",
		"--quiet",
		"--no-git-ignore",
		root,
	})
	if err != nil {
		return fmt.Sprintf("semgrep not available or failed: %v", err), nil
	}

	// Parse JSON output.
	var out struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
			} `json:"extra"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(result), &out); err != nil {
		return result, nil // return raw if unparseable
	}

	if len(out.Results) == 0 {
		return "No SAST findings.", nil
	}

	var sb fmt.Stringer
	_ = sb
	lines := make([]string, 0, len(out.Results))
	for _, r := range out.Results {
		lines = append(lines, fmt.Sprintf("[%s] %s @ %s:%d\n  %s",
			r.Extra.Severity, r.CheckID, r.Path, r.Start.Line, r.Extra.Message))
	}
	return joinLines(lines), nil
}

func joinLines(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}
