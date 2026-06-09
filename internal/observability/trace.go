package observability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// RunTrace captures a full audit run's prompts, tool calls, and summary for replay/debugging.
type RunTrace struct {
	RunID    string        `json:"run_id"`
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Source   string        `json:"source"`
	Events   []TraceEvent  `json:"events"`
}

type TraceEvent struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"` // "prompt" | "tool_call" | "tool_result" | "finding"
	Agent   string    `json:"agent,omitempty"`
	Content string    `json:"content"`
}

// Tracer accumulates events and writes a final trace file.
type Tracer struct {
	trace RunTrace
	dir   string
}

func NewTracer(dir, runID, source string) *Tracer {
	return &Tracer{
		dir: filepath.Join(dir, runID),
		trace: RunTrace{
			RunID:  runID,
			Start:  time.Now(),
			Source: source,
		},
	}
}

func (t *Tracer) Add(kind, agent, content string) {
	t.trace.Events = append(t.trace.Events, TraceEvent{
		At:      time.Now(),
		Kind:    kind,
		Agent:   agent,
		Content: content,
	})
}

func (t *Tracer) Flush() error {
	t.trace.End = time.Now()
	os.MkdirAll(t.dir, 0o755) //nolint:errcheck
	path := filepath.Join(t.dir, "trace.json")
	data, err := json.MarshalIndent(t.trace, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
