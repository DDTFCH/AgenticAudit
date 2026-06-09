package llm

import (
	"fmt"

	"github.com/ddtfch/codeaudit/internal/config"
)

// Role constants used by the router.
const (
	RoleTriage    = "triage"
	RoleReasoning = "reasoning"
	RoleReport    = "report"
)

// Router maps agent roles to concrete Provider instances.
type Router struct {
	providers map[string]Provider
}

// NewRouter builds a Router from the config, constructing each unique provider once.
func NewRouter(cfg *config.ModelsConfig) (*Router, error) {
	instances := make(map[string]Provider) // keyed by provider name

	for _, pc := range cfg.Providers {
		p, err := buildProvider(pc)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", pc.Name, err)
		}
		instances[pc.Name] = p
	}

	r := &Router{providers: make(map[string]Provider)}
	for role, rc := range cfg.Roles {
		p, ok := instances[rc.Provider]
		if !ok {
			return nil, fmt.Errorf("role %q references unknown provider %q", role, rc.Provider)
		}
		// Wrap in a model-pinned adapter so each role can have a distinct model string.
		r.providers[role] = &pinnedProvider{Provider: p, model: rc.Model}
	}

	return r, nil
}

// For returns the Provider for a given role, falling back to reasoning if missing.
func (r *Router) For(role string) (Provider, error) {
	if p, ok := r.providers[role]; ok {
		return p, nil
	}
	if p, ok := r.providers[RoleReasoning]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("no provider configured for role %q", role)
}

// pinnedProvider wraps a Provider and overrides the model identifier.
// The underlying Chat implementation reads the model from context or a struct field;
// we carry it here so each role can use its own model.
type pinnedProvider struct {
	Provider
	model string
}

func (p *pinnedProvider) Name() string {
	return fmt.Sprintf("%s/%s", p.Provider.Name(), p.model)
}

func buildProvider(pc config.ProviderConfig) (Provider, error) {
	switch pc.Type {
	case "anthropic":
		return NewAnthropicProvider(pc)
	case "openai":
		return NewOpenAIProvider(pc)
	case "gemini":
		return NewGeminiProvider(pc)
	case "openai_compatible":
		return NewOpenAICompatibleProvider(pc)
	default:
		return nil, fmt.Errorf("unknown provider type %q", pc.Type)
	}
}
