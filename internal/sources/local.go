package sources

import (
	"context"
	"fmt"
	"os"
)

// LocalSource treats a local directory as the audit target (read-only intent).
type LocalSource struct {
	Path string
}

func (s *LocalSource) Fetch(_ context.Context) (string, func(), error) {
	info, err := os.Stat(s.Path)
	if err != nil {
		return "", noop, fmt.Errorf("local source %q: %w", s.Path, err)
	}
	if !info.IsDir() {
		return "", noop, fmt.Errorf("local source %q is not a directory", s.Path)
	}
	// Return the path directly — no copy, read-only by convention.
	return s.Path, noop, nil
}
