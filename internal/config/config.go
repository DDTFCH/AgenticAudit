package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ProviderConfig struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"` // anthropic | openai | gemini | openai_compatible
	APIKeyEnv string `yaml:"api_key_env"`
	BaseURL   string `yaml:"base_url,omitempty"`
}

type RoleConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type ModelsConfig struct {
	Providers []ProviderConfig       `yaml:"providers"`
	Roles     map[string]*RoleConfig `yaml:"roles"`
}

type ScopeConfig struct {
	Include            []string `yaml:"include"`
	Exclude            []string `yaml:"exclude"`
	FollowDependencies bool     `yaml:"follow_dependencies"`
}

type ToolConfig struct {
	Enabled  bool     `yaml:"enabled"`
	External []string `yaml:"external"`
}

type ToolsConfig struct {
	Secrets      ToolConfig `yaml:"secrets"`
	Dependencies ToolConfig `yaml:"dependencies"`
	SAST         ToolConfig `yaml:"sast"`
}

type GuardrailsConfig struct {
	MaxFiles               int      `yaml:"max_files"`
	MaxRuntimeMinutes      int      `yaml:"max_runtime_minutes"`
	MaxCostUSD             float64  `yaml:"max_cost_usd"`
	EgressAllowlist        []string `yaml:"egress_allowlist"`
	RequireHumanApprovalFor []string `yaml:"require_human_approval_for"`
}

type OutputConfig struct {
	ReportPath   string `yaml:"report_path"`
	ArtifactsDir string `yaml:"artifacts_dir"`
}

// PentestConfig controls the optional autonomous pentest phase (requires cai-framework).
type PentestConfig struct {
	// Port the CAI HTTP API server will listen on (default 18765).
	Port int `yaml:"port"`
	// APIKey for the CAI server (falls back to ALIAS_API_KEY env var).
	APIKey string `yaml:"api_key_env"`
	// StaticAgents are the CAI agent names used for code analysis.
	StaticAgents []string `yaml:"static_agents"`
	// DynamicAgents are the CAI agent names used for live-target testing.
	DynamicAgents []string `yaml:"dynamic_agents"`
	// TimeoutMinutes caps the entire pentest phase duration.
	TimeoutMinutes int `yaml:"timeout_minutes"`
}

type Config struct {
	Models     ModelsConfig     `yaml:"models"`
	Scope      ScopeConfig      `yaml:"scope"`
	Tools      ToolsConfig      `yaml:"tools"`
	Guardrails GuardrailsConfig `yaml:"guardrails"`
	Output     OutputConfig     `yaml:"output"`
	Pentest    PentestConfig    `yaml:"pentest"`
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	cfg.expandEnvKeys()
	return cfg, nil
}

func (c *Config) expandEnvKeys() {
	for i := range c.Models.Providers {
		p := &c.Models.Providers[i]
		if p.APIKeyEnv != "" {
			if v := os.Getenv(p.APIKeyEnv); v != "" {
				_ = v // key resolved at provider init time
			}
		}
	}
}

// ProviderEnvMap returns a map of well-known LLM provider env var names → their
// resolved values, for injection into the CAI subprocess environment.
func (c *Config) ProviderEnvMap() map[string]string {
	m := make(map[string]string)
	knownKeys := []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY",
		"GOOGLE_API_KEY", "ALIAS_API_KEY",
	}
	for _, k := range knownKeys {
		if v := os.Getenv(k); v != "" {
			m[k] = v
		}
	}
	// Also resolve keys declared in the config.
	for _, p := range c.Models.Providers {
		if p.APIKeyEnv != "" {
			if v := os.Getenv(p.APIKeyEnv); v != "" {
				m[p.APIKeyEnv] = v
			}
		}
	}
	return m
}

func (c *Config) ProviderByName(name string) *ProviderConfig {
	for i := range c.Models.Providers {
		if c.Models.Providers[i].Name == name {
			return &c.Models.Providers[i]
		}
	}
	return nil
}

func defaultConfig() *Config {
	return &Config{
		Scope: ScopeConfig{
			Include: []string{"**/*.go", "**/*.py", "**/*.js", "**/*.ts", "**/*.env*", "**/*.yaml"},
			Exclude: []string{"**/node_modules/**", "**/vendor/**", "**/.git/**"},
		},
		Tools: ToolsConfig{
			Secrets:      ToolConfig{Enabled: true},
			Dependencies: ToolConfig{Enabled: true},
			SAST:         ToolConfig{Enabled: false},
		},
		Guardrails: GuardrailsConfig{
			MaxFiles:          5000,
			MaxRuntimeMinutes: 30,
			MaxCostUSD:        10,
			EgressAllowlist:   []string{"github.com", "raw.githubusercontent.com"},
		},
		Output: OutputConfig{
			ReportPath:   "./report.html",
			ArtifactsDir: "./.codeaudit/runs",
		},
		Pentest: PentestConfig{
			Port:           18765,
			StaticAgents:   []string{"Code Agent"},
			DynamicAgents:  []string{"Red Team Agent", "Web Pentester"},
			TimeoutMinutes: 20,
		},
	}
}
