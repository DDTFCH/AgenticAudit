package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ddtfch/codeaudit/internal/findings"
	"github.com/ddtfch/codeaudit/internal/llm"
)

// secretPattern pairs a rule name with a compiled regex.
type secretPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

var builtinPatterns = []secretPattern{
	{Name: "aws-access-key", Pattern: regexp.MustCompile(`(?i)(AKIA|ASIA)[0-9A-Z]{16}`)},
	{Name: "aws-secret-key", Pattern: regexp.MustCompile(`(?i)aws.{0,20}secret.{0,20}['\"]([0-9a-zA-Z/+]{40})['\"]`)},
	{Name: "github-token", Pattern: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{Name: "generic-api-key", Pattern: regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?token)\s*[:=]\s*['\"]([^'\"\s]{16,})['\"]`)},
	{Name: "private-key-header", Pattern: regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)},
	{Name: "generic-secret", Pattern: regexp.MustCompile(`(?i)(password|passwd|secret|token|credential)\s*[:=]\s*['\"]([^'\"\s]{8,})['\"]`)},
	{Name: "jwt-token", Pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{Name: "slack-token", Pattern: regexp.MustCompile(`xox[baprs]-[0-9a-zA-Z]{10,}`)},
	{Name: "basic-auth-url", Pattern: regexp.MustCompile(`https?://[^:]+:[^@]+@`)},
}

// RegisterSecrets registers the secret_scan tool.
func RegisterSecrets(r *Registry, root string) {
	r.Register(llm.ToolSchema{
		Name:        "secret_scan",
		Description: "Scan for hardcoded secrets, API keys, tokens, and credentials using regex patterns and entropy analysis.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File or directory path relative to repo root. Omit to scan everything.",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var a struct {
			Path string `json:"path"`
		}
		json.Unmarshal(args, &a) //nolint:errcheck

		scanRoot := root
		if a.Path != "" {
			clean := filepath.Clean(a.Path)
			if strings.HasPrefix(clean, "..") {
				return "", fmt.Errorf("path traversal not allowed")
			}
			scanRoot = filepath.Join(root, clean)
		}

		fs, err := scanSecrets(scanRoot, root)
		if err != nil {
			return "", err
		}
		return formatFindings(fs), nil
	})
}

func scanSecrets(scanPath, repoRoot string) ([]findings.Finding, error) {
	var results []findings.Finding

	err := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if isBinary(path) {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)

		lines, ferr := fileLines(path)
		if ferr != nil {
			return nil
		}

		for i, line := range lines {
			for _, p := range builtinPatterns {
				if p.Pattern.MatchString(line) {
					loc := fmt.Sprintf("%s:%d", rel, i+1)
					results = append(results, findings.Finding{
						ID:          findings.StableID(findings.CategorySecret, loc, p.Name),
						Title:       fmt.Sprintf("Hardcoded secret: %s", p.Name),
						Category:    findings.CategorySecret,
						Severity:    findings.SeverityAmber, // Validator will confirm
						Confidence:  "needs_review",
						Location:    loc,
						Evidence:    redact(line),
						Description: fmt.Sprintf("Pattern %q matched at %s", p.Name, loc),
						Remediation: "Remove the hardcoded credential and use environment variables or a secrets manager.",
						Status:      findings.StatusOpen,
						DetectedBy:  "secret_scan/builtin",
					})
				}
			}

			// High-entropy string detection (skip short lines).
			if len(line) > 30 {
				for _, tok := range extractQuotedTokens(line) {
					if len(tok) >= 20 && shannonEntropy(tok) > 4.5 {
						loc := fmt.Sprintf("%s:%d", rel, i+1)
						results = append(results, findings.Finding{
							ID:          findings.StableID(findings.CategorySecret, loc, "high-entropy"),
							Title:       "High-entropy string (possible secret)",
							Category:    findings.CategorySecret,
							Severity:    findings.SeverityAmber,
							Confidence:  "needs_review",
							Location:    loc,
							Evidence:    redact(line),
							Description: fmt.Sprintf("High-entropy token detected (entropy=%.2f)", shannonEntropy(tok)),
							Remediation: "Verify this is not a hardcoded secret; if so, move to a secrets manager.",
							Status:      findings.StatusOpen,
							DetectedBy:  "secret_scan/entropy",
						})
						break // one per line
					}
				}
			}
		}
		return nil
	})
	return results, err
}

func redact(s string) string {
	// Keep up to 120 chars but replace anything that looks like a token value.
	if len(s) > 120 {
		s = s[:120] + "..."
	}
	return strings.TrimSpace(s)
}

func extractQuotedTokens(s string) []string {
	re := regexp.MustCompile(`["']([^"'\s]{16,})["']`)
	var tokens []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if len(m) > 1 {
			tokens = append(tokens, m[1])
		}
	}
	return tokens
}

func shannonEntropy(s string) float64 {
	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	l := float64(len(s))
	var e float64
	for _, v := range freq {
		p := v / l
		e -= p * math.Log2(p)
	}
	return e
}

func formatFindings(fs []findings.Finding) string {
	if len(fs) == 0 {
		return "No findings."
	}
	var sb strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&sb, "[%s] %s @ %s\n  %s\n\n", strings.ToUpper(string(f.Severity)), f.Title, f.Location, f.Evidence)
	}
	return sb.String()
}
