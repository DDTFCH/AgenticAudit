package orchestrator

import (
	"fmt"
	"time"
)

// Budget tracks and enforces run guardrails.
type Budget struct {
	MaxFiles     int
	MaxRuntime   time.Duration
	MaxCostUSD   float64
	startTime    time.Time
	filesSeen    int
	totalTokens  int
	estimatedUSD float64
}

func NewBudget(maxFiles, maxMinutes int, maxCostUSD float64) *Budget {
	return &Budget{
		MaxFiles:   maxFiles,
		MaxRuntime: time.Duration(maxMinutes) * time.Minute,
		MaxCostUSD: maxCostUSD,
		startTime:  time.Now(),
	}
}

// RecordFiles adds to the file count.
func (b *Budget) RecordFiles(n int) { b.filesSeen += n }

// RecordTokens adds tokens and estimates cost (rough Anthropic/OpenAI pricing).
func (b *Budget) RecordTokens(inputTokens, outputTokens int) {
	b.totalTokens += inputTokens + outputTokens
	// ~$3/M input + $15/M output (Sonnet-class rough estimate).
	b.estimatedUSD += float64(inputTokens)*3e-6 + float64(outputTokens)*15e-6
}

// Check returns an error if any limit is exceeded.
func (b *Budget) Check() error {
	if b.MaxFiles > 0 && b.filesSeen > b.MaxFiles {
		return fmt.Errorf("file limit exceeded: %d > %d", b.filesSeen, b.MaxFiles)
	}
	if b.MaxRuntime > 0 && time.Since(b.startTime) > b.MaxRuntime {
		return fmt.Errorf("runtime limit exceeded: %v", b.MaxRuntime)
	}
	if b.MaxCostUSD > 0 && b.estimatedUSD > b.MaxCostUSD {
		return fmt.Errorf("cost limit exceeded: $%.2f > $%.2f", b.estimatedUSD, b.MaxCostUSD)
	}
	return nil
}

func (b *Budget) ElapsedSeconds() float64 { return time.Since(b.startTime).Seconds() }
func (b *Budget) EstimatedCostUSD() float64 { return b.estimatedUSD }
func (b *Budget) TotalTokens() int { return b.totalTokens }
