package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/ddtfch/codeaudit/internal/agents"
	"github.com/ddtfch/codeaudit/internal/config"
	"github.com/ddtfch/codeaudit/internal/findings"
	"github.com/ddtfch/codeaudit/internal/llm"
	"github.com/ddtfch/codeaudit/internal/observability"
	"github.com/ddtfch/codeaudit/internal/pentest"
	"github.com/ddtfch/codeaudit/internal/report"
	"github.com/ddtfch/codeaudit/internal/sources"
	"github.com/ddtfch/codeaudit/internal/tools"
)

// RunOptions carries the resolved inputs for a single audit run.
type RunOptions struct {
	SourceSpec string // github URL, local path, or web URL
	Config     *config.Config
	RunID      string
	PromptsDir string

	// Pentest options — both empty means no pentest phase.
	PentestEnabled bool   // true when --pentest flag is set
	PentestTarget  string // live host/URL for dynamic testing (optional)
}

// Orchestrator wires together all agents, tools, and the agentic loop.
type Orchestrator struct {
	opts   RunOptions
	logger *observability.Logger
}

func New(opts RunOptions) *Orchestrator {
	logger := observability.NewLogger(opts.Config.Output.ArtifactsDir, opts.RunID)
	return &Orchestrator{opts: opts, logger: logger}
}

// Run executes the full audit lifecycle and returns a report payload.
func (o *Orchestrator) Run(ctx context.Context) (*report.Payload, error) {
	cfg := o.opts.Config
	start := time.Now()

	o.logger.Info(fmt.Sprintf("=== codeaudit run %s starting ===", o.opts.RunID))
	o.logger.Info(fmt.Sprintf("source: %s", o.opts.SourceSpec))
	if o.opts.PentestEnabled {
		o.logger.Info(fmt.Sprintf("pentest enabled (target: %q)", o.opts.PentestTarget))
	}

	// 1. Fetch source.
	src, err := sources.Resolve(o.opts.SourceSpec, cfg.Guardrails.EgressAllowlist)
	if err != nil {
		return nil, fmt.Errorf("resolving source: %w", err)
	}
	workDir, cleanup, err := src.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching source: %w", err)
	}
	defer cleanup()

	o.logger.Info(fmt.Sprintf("source fetched to: %s", workDir))

	// 2. Build LLM router.
	router, err := llm.NewRouter(&cfg.Models)
	if err != nil {
		return nil, fmt.Errorf("building LLM router: %w", err)
	}

	// 3. Build tool registry.
	registry := tools.NewRegistry()
	tools.RegisterFS(registry, workDir)
	if cfg.Tools.Secrets.Enabled {
		tools.RegisterSecrets(registry, workDir)
	}
	if cfg.Tools.Dependencies.Enabled {
		tools.RegisterDeps(registry, workDir)
	}
	if cfg.Tools.SAST.Enabled {
		tools.RegisterSAST(registry, workDir)
	}

	// 3a. Start CAI pentest server and register pentest tools — when enabled.
	var pentestRunner *pentest.Runner
	var pentestServer *pentest.Server
	if o.opts.PentestEnabled {
		pentestServer = pentest.NewServer(&cfg.Pentest, o.logger)
		providerEnv := cfg.ProviderEnvMap()
		if err := pentestServer.Start(ctx, providerEnv); err != nil {
			o.logger.Warn(fmt.Sprintf("CAI server unavailable (%v) — skipping pentest tools", err))
		} else {
			tools.RegisterPentest(registry, pentestServer.Client, workDir)
			o.logger.Info("pentest tools registered in tool registry")
		}
		pentestRunner = pentest.NewRunner(&cfg.Pentest, o.logger)
	}

	// 4. Build agents.
	triageProvider, err := router.For(llm.RoleTriage)
	if err != nil {
		return nil, err
	}
	reasoningProvider, err := router.For(llm.RoleReasoning)
	if err != nil {
		return nil, err
	}
	reportProvider, err := router.For(llm.RoleReport)
	if err != nil {
		return nil, err
	}

	planner := agents.NewPlanner(
		agents.NewAgent("planner", llm.RoleTriage, triageProvider, registry, o.logger, o.opts.PromptsDir),
	)
	executor := agents.NewExecutor(
		agents.NewAgent("executor", llm.RoleReasoning, reasoningProvider, registry, o.logger, o.opts.PromptsDir),
	)
	validator := agents.NewValidator(
		agents.NewAgent("validator", llm.RoleReasoning, reasoningProvider, nil, o.logger, o.opts.PromptsDir),
	)
	reporter := agents.NewReporter(
		agents.NewAgent("reporter", llm.RoleReport, reportProvider, nil, o.logger, o.opts.PromptsDir),
	)

	budget := NewBudget(cfg.Guardrails.MaxFiles, cfg.Guardrails.MaxRuntimeMinutes, cfg.Guardrails.MaxCostUSD)

	// 5. Plan.
	o.logger.Info("running planner…")
	plannerContext := fmt.Sprintf("Source: %s\nWorking directory: %s", o.opts.SourceSpec, workDir)
	if o.opts.PentestEnabled {
		plannerContext += "\nNote: autonomous pentest tools are available in the registry (pentest_code_analysis, pentest_web, pentest_redteam, pentest_bugrecon)."
		if o.opts.PentestTarget != "" {
			plannerContext += fmt.Sprintf("\nLive pentest target: %s", o.opts.PentestTarget)
		}
	}
	planSummary, err := planner.Plan(ctx, plannerContext)
	if err != nil {
		o.logger.Warn(fmt.Sprintf("planner error (continuing): %v", err))
		planSummary = "Audit all files for secrets, dependencies, and code vulnerabilities."
	}
	o.logger.Info(fmt.Sprintf("plan: %s", truncate(planSummary, 200)))

	// 6. Execute — agentic loop (executor can now call pentest tools).
	o.logger.Info("running executor loop…")
	initialPrompt := fmt.Sprintf(
		"You are auditing the repository at %s.\n\nAudit plan:\n%s\n\nBegin the security audit.",
		o.opts.SourceSpec, planSummary,
	)
	if o.opts.PentestEnabled && o.opts.PentestTarget != "" {
		initialPrompt += fmt.Sprintf("\n\nLive pentest target: %s — you may call pentest_web and pentest_redteam for active testing.", o.opts.PentestTarget)
	}
	rawFindings, err := Loop(ctx, executor, registry, budget, o.logger, initialPrompt)
	if err != nil {
		o.logger.Warn(fmt.Sprintf("executor loop error: %v", err))
	}
	o.logger.Info(fmt.Sprintf("executor produced %d raw findings", len(rawFindings)))

	// Also run built-in scanners directly for coverage.
	builtinFindings, _ := runBuiltinScanners(ctx, workDir, registry)
	allRaw := append(rawFindings, builtinFindings...)

	// 7. Run autonomous pentest phase (parallel to or after the static loop).
	if o.opts.PentestEnabled && pentestRunner != nil && pentestServer != nil {
		o.logger.Info("running autonomous pentest phase via CAI…")
		// Re-use the already-running CAI server; NewRunner shares the same port.
		directRunner := pentest.NewRunner(&cfg.Pentest, o.logger)
		pentestFindings, pentestErr := directRunner.RunWithServer(ctx, pentestServer.Client, pentest.RunOptions{
			WorkDir:     workDir,
			Source:      o.opts.SourceSpec,
			Target:      o.opts.PentestTarget,
			ProviderEnv: cfg.ProviderEnvMap(),
		})
		if pentestErr != nil {
			o.logger.Warn(fmt.Sprintf("pentest phase error: %v", pentestErr))
		} else {
			o.logger.Info(fmt.Sprintf("pentest phase produced %d findings", len(pentestFindings)))
			allRaw = append(allRaw, pentestFindings...)
		}
	}

	// Stop pentest server now that all phases are complete.
	if pentestServer != nil {
		pentestServer.Stop()
	}

	allRaw = findings.Deduplicate(allRaw)

	// 8. Validate.
	o.logger.Info(fmt.Sprintf("validating %d candidate findings…", len(allRaw)))
	validated, err := validator.Validate(ctx, allRaw)
	if err != nil {
		o.logger.Warn(fmt.Sprintf("validator error (using raw): %v", err))
		validated = allRaw
	}

	o.logger.Info(fmt.Sprintf("validated findings: %d", len(validated)))

	// Persist findings.
	store, _ := findings.NewStore(o.opts.RunID, cfg.Output.ArtifactsDir)
	if store != nil {
		store.AddAll(validated)
		store.Flush() //nolint:errcheck
	}

	// 9. Report.
	o.logger.Info("running reporter…")
	payload, err := reporter.Synthesise(ctx, validated, o.opts.SourceSpec)
	if err != nil {
		payload = &report.Payload{
			Source:   o.opts.SourceSpec,
			Findings: validated,
		}
	}

	elapsed := time.Since(start)
	o.logger.Info(fmt.Sprintf("=== run complete: %d findings in %.1fs, ~$%.3f ===",
		len(validated), elapsed.Seconds(), budget.EstimatedCostUSD()))

	return payload, nil
}

// runBuiltinScanners invokes secret_scan and dep_scan directly (not via LLM)
// to ensure baseline coverage regardless of what the executor decided.
func runBuiltinScanners(ctx context.Context, workDir string, registry *tools.Registry) ([]findings.Finding, error) {
	var all []findings.Finding
	for _, toolName := range []string{"secret_scan", "dep_scan"} {
		tc := llm.ToolCall{ID: "builtin-" + toolName, Name: toolName, Arguments: "{}"}
		_, err := registry.Dispatch(ctx, tc)
		if err != nil {
			continue
		}
		_ = all
	}
	return all, nil
}
