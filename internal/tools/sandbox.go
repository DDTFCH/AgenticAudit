package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const sandboxTimeout = 5 * time.Minute

// runSandboxed executes an external tool (gitleaks, semgrep, osv-scanner) in a
// constrained subprocess. The working directory is set to workDir (read-only intent).
// stdout is returned; stderr is suppressed unless the command fails.
func runSandboxed(ctx context.Context, workDir, name string, args []string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found in PATH", name)
	}

	ctx, cancel := context.WithTimeout(ctx, sandboxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s timed out", name)
		}
		// Some scanners (semgrep) return non-zero exit codes when findings exist.
		if stdout.Len() > 0 {
			return stdout.String(), nil
		}
		return "", fmt.Errorf("%s failed: %v — %s", name, err, stderr.String())
	}
	return stdout.String(), nil
}
