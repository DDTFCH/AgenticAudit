package skills

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed packs/secrets.yaml
var secretsYAML string

//go:embed packs/dependencies.yaml
var depsYAML string

//go:embed packs/remediation.md
var remediationMD string

// Pack is a named knowledge document exposed to agents.
type Pack struct {
	Name    string
	Content string
}

// LoadAll returns all embedded knowledge packs.
func LoadAll() []Pack {
	return []Pack{
		{Name: "secrets", Content: secretsYAML},
		{Name: "dependencies", Content: depsYAML},
		{Name: "remediation", Content: remediationMD},
	}
}

// LoadFromDir loads additional packs from a directory, merging with embedded ones.
func LoadFromDir(dir string) ([]Pack, error) {
	packs := LoadAll()
	if dir == "" {
		return packs, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return packs, fmt.Errorf("reading skills dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, ferr := os.ReadFile(filepath.Join(dir, e.Name()))
		if ferr != nil {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		packs = append(packs, Pack{Name: name, Content: string(data)})
	}
	return packs, nil
}

// Format returns all packs as a single string suitable for inclusion in a system prompt.
func Format(packs []Pack) string {
	var sb strings.Builder
	for _, p := range packs {
		fmt.Fprintf(&sb, "## Knowledge: %s\n\n%s\n\n", p.Name, p.Content)
	}
	return sb.String()
}
