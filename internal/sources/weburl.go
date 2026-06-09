package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// WebURLSource downloads files from a web URL (e.g. a raw GitHub folder listing).
// Only hosts in the Allowlist are permitted (egress guardrail).
type WebURLSource struct {
	URL       string
	Allowlist []string
}

func (s *WebURLSource) Fetch(ctx context.Context) (string, func(), error) {
	if err := s.checkAllowlist(s.URL); err != nil {
		return "", noop, err
	}

	dir, err := os.MkdirTemp("", "codeaudit-web-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { os.RemoveAll(dir) }

	// Try to download a single file.
	dest := filepath.Join(dir, filepath.Base(s.URL))
	if err := s.download(ctx, s.URL, dest); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("downloading %s: %w", s.URL, err)
	}

	return dir, cleanup, nil
}

func (s *WebURLSource) download(ctx context.Context, rawURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func (s *WebURLSource) checkAllowlist(rawURL string) error {
	if len(s.Allowlist) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range s.Allowlist {
		if host == strings.ToLower(allowed) || strings.HasSuffix(host, "."+strings.ToLower(allowed)) {
			return nil
		}
	}
	return fmt.Errorf("host %q not in egress allowlist %v", host, s.Allowlist)
}
