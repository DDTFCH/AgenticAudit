package sources

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitHubSource clones a GitHub repository to a temp directory.
// RepoRef format: "owner/repo" or "owner/repo@branch"
type GitHubSource struct {
	RepoRef string
	Token   string // optional; reads GITHUB_TOKEN env if empty
}

func (s *GitHubSource) Fetch(ctx context.Context) (string, func(), error) {
	ref := s.RepoRef
	branch := ""
	if idx := strings.Index(ref, "@"); idx >= 0 {
		branch = ref[idx+1:]
		ref = ref[:idx]
	}

	token := s.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	repoURL := fmt.Sprintf("https://github.com/%s.git", ref)
	if token != "" {
		repoURL = fmt.Sprintf("https://%s@github.com/%s.git", token, ref)
	}

	dir, err := os.MkdirTemp("", "codeaudit-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { os.RemoveAll(dir) }

	args := []string{"clone", "--depth=1", "--single-branch"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, dir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("git clone %s: %w\n%s", ref, err, out)
	}

	return dir, cleanup, nil
}

func noop() {}
