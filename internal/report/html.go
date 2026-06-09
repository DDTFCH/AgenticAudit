package report

import (
	"bytes"
	_ "embed"
	"html/template"
	"os"
	"path/filepath"
)

//go:embed templates/report.html.tmpl
var reportTemplate string

// Render writes the self-contained HTML report to outPath.
func Render(p *Payload, outPath string) error {
	p.Finalise()

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"upper": func(s string) string {
			if len(s) == 0 {
				return s
			}
			return string(s[0]-32) + s[1:]
		},
		"add": func(a, b int) int { return a + b },
	}).Parse(reportTemplate)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}
