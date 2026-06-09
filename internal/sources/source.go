package sources

import (
	"context"
	"fmt"
	"strings"
)

// Source abstracts the origin of the code to be audited.
type Source interface {
	// Fetch downloads/copies the source to a temporary working directory.
	// The caller must invoke the returned cleanup function when done.
	Fetch(ctx context.Context) (workDir string, cleanup func(), err error)
}

// Resolve picks the correct Source implementation from a spec string.
// Spec can be:
//   - "github:owner/repo[@ref]"  — GitHub clone
//   - "https://..." / "http://..."  — web URL (file listing)
//   - anything else               — local filesystem path
func Resolve(spec string, egressAllowlist []string) (Source, error) {
	switch {
	case strings.HasPrefix(spec, "github:"):
		ref := strings.TrimPrefix(spec, "github:")
		return &GitHubSource{RepoRef: ref}, nil
	case strings.HasPrefix(spec, "https://") || strings.HasPrefix(spec, "http://"):
		return &WebURLSource{URL: spec, Allowlist: egressAllowlist}, nil
	case spec == "":
		return nil, fmt.Errorf("source spec is empty")
	default:
		return &LocalSource{Path: spec}, nil
	}
}
