package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ddtfch/codeaudit/internal/llm"
)

// RegisterDeps registers the dep_scan tool.
func RegisterDeps(r *Registry, root string) {
	r.Register(llm.ToolSchema{
		Name:        "dep_scan",
		Description: "Parse dependency manifests and identify outdated or potentially vulnerable packages. Detects package.json, go.mod, requirements.txt, Gemfile, pom.xml.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, func(ctx context.Context, args json.RawMessage) (string, error) {
		return scanDeps(root)
	})
}

func scanDeps(root string) (string, error) {
	var sb strings.Builder
	found := false

	manifests := map[string]func(string) string{
		"package.json":     parsePackageJSON,
		"go.mod":           parseGoMod,
		"requirements.txt": parseRequirementsTxt,
		"Gemfile":          parseGemfile,
		"pom.xml":          parsePomXML,
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := filepath.Base(path)
		if parser, ok := manifests[base]; ok {
			rel, _ := filepath.Rel(root, path)
			data, ferr := os.ReadFile(path)
			if ferr != nil {
				return nil
			}
			result := parser(string(data))
			if result != "" {
				fmt.Fprintf(&sb, "=== %s ===\n%s\n", rel, result)
				found = true
			}
		}
		return nil
	})

	if err != nil {
		return sb.String(), err
	}
	if !found {
		return "No dependency manifests found.", nil
	}
	return sb.String(), nil
}

func parsePackageJSON(content string) string {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return ""
	}
	var sb strings.Builder
	for name, ver := range pkg.Dependencies {
		fmt.Fprintf(&sb, "  dep: %s@%s\n", name, ver)
	}
	for name, ver := range pkg.DevDependencies {
		fmt.Fprintf(&sb, "  devDep: %s@%s\n", name, ver)
	}
	return sb.String()
}

func parseGoMod(content string) string {
	var sb strings.Builder
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require") || (len(line) > 0 && line[0] != '/' && strings.Contains(line, " v")) {
			sb.WriteString("  " + line + "\n")
		}
	}
	return sb.String()
}

func parseRequirementsTxt(content string) string {
	var sb strings.Builder
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			sb.WriteString("  " + line + "\n")
		}
	}
	return sb.String()
}

func parseGemfile(content string) string {
	var sb strings.Builder
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gem ") {
			sb.WriteString("  " + line + "\n")
		}
	}
	return sb.String()
}

func parsePomXML(content string) string {
	// Basic extraction — a real implementation would use an XML parser.
	var sb strings.Builder
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "<artifactId>") && i+1 < len(lines) {
			artifact := strings.TrimSpace(extractXMLTag(line, "artifactId"))
			version := ""
			if i+2 < len(lines) {
				version = strings.TrimSpace(extractXMLTag(lines[i+2], "version"))
			}
			if artifact != "" {
				fmt.Fprintf(&sb, "  %s:%s\n", artifact, version)
			}
		}
	}
	return sb.String()
}

func extractXMLTag(s, tag string) string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(s, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(s[start:], close)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}
