package findings

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

type Severity string
type Status string
type Category string

const (
	SeverityRed   Severity = "red"
	SeverityAmber Severity = "amber"
	SeverityGreen Severity = "green"

	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusFixed      Status = "fixed"
	StatusRetest     Status = "retest"
	StatusAccepted   Status = "accepted"

	CategorySecret       Category = "secret"
	CategoryDependency   Category = "dependency"
	CategoryVulnerability Category = "vulnerability"
	CategoryMisconfig    Category = "misconfig"
)

type Finding struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    Category `json:"category"`
	Severity    Severity `json:"severity"`
	Confidence  string   `json:"confidence"` // confirmed | likely | needs_review
	Location    string   `json:"location"`
	Evidence    string   `json:"evidence"`
	Description string   `json:"description"`
	Remediation string   `json:"remediation"`
	Refs        []string `json:"refs"`
	Status      Status   `json:"status"`
	DetectedBy  string   `json:"detected_by"`
}

// StableID generates a deterministic ID from category + location + a rule/title slug.
func StableID(category Category, location, ruleSlug string) string {
	h := sha256.Sum256([]byte(string(category) + "|" + location + "|" + ruleSlug))
	return fmt.Sprintf("%x", h[:8])
}

// MapSeverity converts a raw severity level + confidence to a RAG color.
func MapSeverity(rawLevel string, confidence string) Severity {
	level := strings.ToLower(rawLevel)
	conf := strings.ToLower(confidence)

	switch level {
	case "critical", "high":
		if conf == "confirmed" {
			return SeverityRed
		}
		return SeverityAmber
	case "medium":
		return SeverityAmber
	default:
		return SeverityGreen
	}
}
