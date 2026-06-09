package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ddtfch/codeaudit/internal/config"
	"github.com/ddtfch/codeaudit/internal/orchestrator"
	"github.com/ddtfch/codeaudit/internal/report"
)

var (
	flagSource        string
	flagConfig        string
	flagOut           string
	flagPromptsDir    string
	flagVerbose       bool
	flagPentest       bool
	flagPentestTarget string
)

func main() {
	root := &cobra.Command{
		Use:   "codeaudit",
		Short: "Agentic security audit tool for codebases",
		Long: `codeaudit performs an LLM-driven security audit of a codebase,
scanning for secrets, vulnerable dependencies, and code-level risks.
It produces a self-contained HTML report with Red/Amber/Green findings.

Enable autonomous pentesting with --pentest (requires cai-framework installed).
Add --pentest-target <url|host> for active dynamic testing against a live deployment.`,
		RunE: runAudit,
	}

	root.Flags().StringVarP(&flagSource, "source", "s", "", "Source: 'github:owner/repo[@ref]', local path, or https:// URL (required)")
	root.Flags().StringVarP(&flagConfig, "config", "c", "", "Path to codeaudit.yaml config file")
	root.Flags().StringVarP(&flagOut, "out", "o", "", "Output report path (default: ./report.html)")
	root.Flags().StringVar(&flagPromptsDir, "prompts-dir", "", "Directory with custom agent system prompts (*.md)")
	root.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
	root.Flags().BoolVar(&flagPentest, "pentest", false, "Enable autonomous pentest phase via CAI (requires: pip install cai-framework)")
	root.Flags().StringVar(&flagPentestTarget, "pentest-target", "",
		"Live target URL or host for dynamic pentesting, e.g. 'https://myapp.example.com' or '192.168.1.10'.\n"+
			"Only used when --pentest is also set. You must have explicit written authorization to test this target.")

	root.MarkFlagRequired("source") //nolint:errcheck

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAudit(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(flagConfig)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if flagOut != "" {
		cfg.Output.ReportPath = flagOut
	}

	// Safety gate: require explicit acknowledgement for active pentesting.
	if flagPentestTarget != "" && !flagPentest {
		return fmt.Errorf("--pentest-target requires --pentest to be set")
	}

	runID := fmt.Sprintf("%d", time.Now().Unix())

	orc := orchestrator.New(orchestrator.RunOptions{
		SourceSpec:     flagSource,
		Config:         cfg,
		RunID:          runID,
		PromptsDir:     flagPromptsDir,
		PentestEnabled: flagPentest,
		PentestTarget:  flagPentestTarget,
	})

	ctx := context.Background()
	payload, err := orc.Run(ctx)
	if err != nil {
		return fmt.Errorf("audit failed: %w", err)
	}

	if err := report.Render(payload, cfg.Output.ReportPath); err != nil {
		return fmt.Errorf("rendering report: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n✓ Report written to %s\n", cfg.Output.ReportPath)
	fmt.Fprintf(os.Stderr, "  Findings: %d red  %d amber  %d green\n",
		payload.RedCount, payload.AmberCount, payload.GreenCount)

	if flagPentest {
		fmt.Fprintf(os.Stderr, "  Pentest: enabled")
		if flagPentestTarget != "" {
			fmt.Fprintf(os.Stderr, " (target: %s)", flagPentestTarget)
		}
		fmt.Fprintln(os.Stderr)
	}

	return nil
}
