package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ddtfch/codeaudit/internal/llm"
)

// RegisterFS registers list_files, read_file, and grep_file tools into r.
func RegisterFS(r *Registry, root string) {
	r.Register(llm.ToolSchema{
		Name:        "list_files",
		Description: "List files in the target repository, optionally filtered by a glob pattern.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Glob pattern, e.g. '**/*.go'. Omit to list all files."},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var a struct {
			Pattern string `json:"pattern"`
		}
		json.Unmarshal(args, &a) //nolint:errcheck
		pattern := a.Pattern
		if pattern == "" {
			pattern = "**/*"
		}
		matches, err := globDir(root, pattern)
		if err != nil {
			return "", err
		}
		return strings.Join(matches, "\n"), nil
	})

	r.Register(llm.ToolSchema{
		Name:        "read_file",
		Description: "Read the contents of a file in the target repository. For large files, use offset and limit to read in chunks (overlap lines for context continuity).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Path relative to repo root"},
				"offset": map[string]any{"type": "integer", "description": "Line offset (0-based)"},
				"limit":  map[string]any{"type": "integer", "description": "Max lines to return (default 200)"},
			},
			"required": []string{"path"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var a struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		a.Limit = 200
		json.Unmarshal(args, &a) //nolint:errcheck
		clean := filepath.Clean(a.Path)
		if strings.HasPrefix(clean, "..") {
			return "", fmt.Errorf("path traversal not allowed")
		}
		return readFileChunked(filepath.Join(root, clean), a.Offset, a.Limit)
	})

	r.Register(llm.ToolSchema{
		Name:        "grep_file",
		Description: "Search for a regex pattern across files in the repository. Returns matching lines with context.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "Regular expression"},
				"glob":    map[string]any{"type": "string", "description": "File glob filter, e.g. '**/*.py'"},
				"context": map[string]any{"type": "integer", "description": "Lines of context around each match (default 2)"},
			},
			"required": []string{"pattern"},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		var a struct {
			Pattern string `json:"pattern"`
			Glob    string `json:"glob"`
			Context int    `json:"context"`
		}
		a.Context = 2
		json.Unmarshal(args, &a) //nolint:errcheck
		return grepDir(root, a.Pattern, a.Glob, a.Context)
	})
}

func globDir(root, pattern string) ([]string, error) {
	var results []string
	// Determine if it's a simple extension filter (e.g. **/*.go)
	ext := ""
	if strings.Contains(pattern, "*.") {
		ext = filepath.Ext(pattern)
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if pattern == "**/*" {
			results = append(results, rel)
			return nil
		}
		if ext != "" && filepath.Ext(rel) == ext {
			results = append(results, rel)
		}
		return nil
	})
	return results, err
}

func readFileChunked(path string, offset, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		if lineNum >= offset && len(lines) < limit {
			lines = append(lines, fmt.Sprintf("%d: %s", lineNum+1, scanner.Text()))
		}
		lineNum++
		if len(lines) >= limit {
			break
		}
	}
	return strings.Join(lines, "\n"), scanner.Err()
}

func grepDir(root, pattern, glob string, contextLines int) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid pattern: %w", err)
	}

	ext := ""
	if glob != "" && strings.Contains(glob, "*.") {
		ext = filepath.Ext(glob)
	}

	var sb strings.Builder
	matchCount := 0

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if isBinary(path) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if ext != "" && filepath.Ext(rel) != ext {
			return nil
		}

		lines, ferr := fileLines(path)
		if ferr != nil {
			return nil
		}

		for i, line := range lines {
			if re.MatchString(line) {
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines + 1
				if end > len(lines) {
					end = len(lines)
				}
				sb.WriteString(fmt.Sprintf("--- %s:%d ---\n", rel, i+1))
				for j := start; j < end; j++ {
					prefix := "  "
					if j == i {
						prefix = "> "
					}
					sb.WriteString(fmt.Sprintf("%s%d: %s\n", prefix, j+1, lines[j]))
				}
				matchCount++
				if matchCount >= 100 {
					sb.WriteString("... (truncated at 100 matches)\n")
					return errDone
				}
			}
		}
		return nil
	})

	if walkErr != nil && walkErr != errDone {
		return sb.String(), walkErr
	}
	if matchCount == 0 {
		return "(no matches)", nil
	}
	return sb.String(), nil
}

var errDone = fmt.Errorf("done")

func fileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}
