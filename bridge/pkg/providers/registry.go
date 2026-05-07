package providers

import (
	"encoding/json"
	"fmt"
	"os"
)

// Provider represents a single AI provider from the registry
type Provider struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"`
	BaseURL  string   `json:"base_url"`
	Aliases  []string `json:"aliases,omitempty"`
}

// Registry represents the provider registry
type Registry struct {
	Providers []Provider `json:"providers"`

	sealedCheck func() error
	resetTimer  func()
}

// DefaultRegistryPath is the default path to the providers registry
const DefaultRegistryPath = "/etc/armorclaw/providers.json"

// EmbeddedRegistry is the embedded default registry (fallback)
var EmbeddedRegistry = Registry{
	Providers: []Provider{
		{ID: "openai", Name: "OpenAI", Protocol: "openai", BaseURL: "https://api.openai.com/v1"},
		{ID: "anthropic", Name: "Anthropic", Protocol: "anthropic", BaseURL: "https://api.anthropic.com/v1"},
		{ID: "google", Name: "Google", Protocol: "openai", BaseURL: "https://generativelanguage.googleapis.com/v1"},
		{ID: "xai", Name: "xAI", Protocol: "openai", BaseURL: "https://api.x.ai/v1"},
		{ID: "openrouter", Name: "OpenRouter", Protocol: "openai", BaseURL: "https://openrouter.ai/api/v1"},
		{ID: "zhipu", Name: "Zhipu AI (Z AI)", Protocol: "openai", BaseURL: "https://api.z.ai/api/paas/v4", Aliases: []string{"zai", "glm"}},
		{ID: "deepseek", Name: "DeepSeek", Protocol: "openai", BaseURL: "https://api.deepseek.com/v1"},
		{ID: "moonshot", Name: "Moonshot AI", Protocol: "openai", BaseURL: "https://api.moonshot.ai/v1"},
		{ID: "nvidia", Name: "NVIDIA NIM", Protocol: "openai", BaseURL: "https://integrate.api.nvidia.com/v1"},
		{ID: "groq", Name: "Groq", Protocol: "openai", BaseURL: "https://api.groq.com/openai/v1"},
		{ID: "cloudflare", Name: "Cloudflare", Protocol: "openai", BaseURL: "https://gateway.ai.cloudflare.com/v1"},
		{ID: "ollama", Name: "Ollama (Local)", Protocol: "openai", BaseURL: "http://localhost:11434/v1"},
	},
}

// LoadRegistry loads the provider registry from the specified path
func LoadRegistry(registryPath string) (*Registry, error) {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return embedded fallback
			return &EmbeddedRegistry, nil
		}
		return nil, fmt.Errorf("failed to read registry file: %w", err)
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry JSON: %w", err)
	}

	return &registry, nil
}

// GetProviderByID finds a provider by its ID
func (r *Registry) GetProviderByID(id string) (*Provider, bool) {
	for i := range r.Providers {
		p := &r.Providers[i]
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}

// GetProviderByAlias finds a provider by its alias
func (r *Registry) GetProviderByAlias(alias string) (*Provider, bool) {
	for i := range r.Providers {
		p := &r.Providers[i]
		if p.ID == alias {
			return p, true
		}
		if p.Aliases != nil {
			for _, a := range p.Aliases {
				if a == alias {
					return p, true
				}
			}
		}
	}
	return nil, false
}

// ResolveProvider resolves a provider ID or alias to the full Provider struct
func (r *Registry) ResolveProvider(input string) (*Provider, bool) {
	// First try as exact ID match
	if provider, found := r.GetProviderByID(input); found {
		return provider, true
	}

	// Then try as alias match
	if provider, found := r.GetProviderByAlias(input); found {
		return provider, true
	}

	return nil, false
}

// GetAllProviders returns all providers as a map of ID to Provider
func (r *Registry) GetAllProviders() map[string]Provider {
	providers := make(map[string]Provider, len(r.Providers))
	for i := range r.Providers {
		p := &r.Providers[i]
		providers[p.ID] = *p
	}
	return providers
}

// GetProviderCount returns the number of providers in the registry
func (r *Registry) GetProviderCount() int {
	return len(r.Providers)
}

// SetSealedCheck configures the sealed keystore gate for key lookups.
func (r *Registry) SetSealedCheck(check func() error, reset func()) {
	r.sealedCheck = check
	r.resetTimer = reset
}

// LookupProviderKey resolves a provider by ID or alias after verifying
// the keystore is unsealed. This is an activity operation — it resets
// the auto-seal timer on success.
func (r *Registry) LookupProviderKey(providerID string) (*Provider, error) {
	if r.sealedCheck != nil {
		if err := r.sealedCheck(); err != nil {
			return nil, err
		}
	}

	provider, found := r.ResolveProvider(providerID)
	if !found {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	if r.resetTimer != nil {
		r.resetTimer()
	}

	return provider, nil
}
