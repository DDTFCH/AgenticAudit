# codeaudit

A portable, single-binary agentic security audit tool. An LLM orchestration loop drives the audit, calling deterministic tools (filesystem, secret scanner, dependency scanner, optional SAST) to gather evidence, then a Validator agent reduces false positives and a Reporter agent produces a self-contained HTML report with **Red / Amber / Green** findings.

## Quick start

```bash
# Build
make build

# Audit a GitHub repo (needs ANTHROPIC_API_KEY)
export ANTHROPIC_API_KEY=sk-ant-...
./codeaudit --source github:owner/repo

# Audit a local directory
./codeaudit --source /path/to/project --out /tmp/report.html

# Use a custom config
./codeaudit --source github:owner/repo --config configs/codeaudit.example.yaml
```

## Configuration

Copy `configs/codeaudit.example.yaml` and edit to configure:
- **model roles** — which provider/model for triage, reasoning, and report writing
- **scope** — include/exclude globs
- **tools** — enable/disable and point at external scanners
- **guardrails** — max files, runtime, cost cap, egress allowlist
- **output** — report path and artifacts directory

See [docs/architecture.md](docs/architecture.md) for the full design.

## Architecture

```
┌─────────────────────────────────────────┐
│              Orchestrator               │
│  ┌──────────┐  ┌──────────┐            │
│  │ Planner  │  │ Executor │◄──┐        │
│  └──────────┘  └──────────┘   │        │
│       │              │    Tool calls   │
│       ▼              ▼        │        │
│  ┌──────────┐  ┌──────────┐   │        │
│  │Validator │  │ Reporter │   │        │
│  └──────────┘  └──────────┘   │        │
└──────────────────────────────────────────┘
         │               │
    ┌────▼────┐    ┌──────▼──────┐
    │ LLM     │    │  Tool       │
    │ Router  │    │  Registry   │
    │ triage/ │    │ fs/secrets/ │
    │ reason/ │    │ deps/sast   │
    │ report  │    └─────────────┘
    └─────────┘
```

**Agents:**
- **Planner** — scopes the audit, decides what to deep-dive (cheap/local model)
- **Executor** — runs tools, gathers evidence (frontier model)
- **Validator** — false-positive reduction, sets confidence (frontier model)
- **Reporter** — synthesises findings into the HTML report (mid-tier model)

**Supported LLM providers:** Anthropic, OpenAI, Gemini, OpenAI-compatible (Ollama, vLLM, LM Studio, OpenRouter, LiteLLM)

**Report features:** RAG dashboard, sortable/filterable findings table, per-finding remediation status (persisted in `localStorage`), CSV/JSON export, retest support.

## External scanners (optional)

Install any of these to enable richer scanning:

| Scanner | Install | Enables |
|---------|---------|---------|
| [gitleaks](https://github.com/gitleaks/gitleaks) | `brew install gitleaks` | Enhanced secret detection |
| [osv-scanner](https://github.com/google/osv-scanner) | `go install ...` | OSV vulnerability DB |
| [semgrep](https://semgrep.dev) | `pip install semgrep` | SAST rules |

## License

MIT
