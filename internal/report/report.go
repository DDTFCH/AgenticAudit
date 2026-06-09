package report

import (
	"time"

	"github.com/ddtfch/codeaudit/internal/findings"
)

// Payload is the data structure passed to the HTML template.
type Payload struct {
	Source           string
	GeneratedAt      time.Time
	ExecutiveSummary string
	Findings         []findings.Finding

	// Counts derived at render time.
	RedCount   int
	AmberCount int
	GreenCount int
}

// Finalise populates derived counts and timestamps.
func (p *Payload) Finalise() {
	p.GeneratedAt = time.Now()
	p.RedCount = 0
	p.AmberCount = 0
	p.GreenCount = 0
	for _, f := range p.Findings {
		switch f.Severity {
		case findings.SeverityRed:
			p.RedCount++
		case findings.SeverityAmber:
			p.AmberCount++
		default:
			p.GreenCount++
		}
	}
}
