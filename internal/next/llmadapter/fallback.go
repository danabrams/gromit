package llmadapter

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/danabrams/gromit/internal/provider"
)

// RouterSelector abstracts the router's Select and MarkUnavailable methods.
type RouterSelector interface {
	Select(phase string, tier string) (provider.Provider, string)
	MarkUnavailable(name string)
}

// FallbackAdapter wraps provider selection with automatic fallback on usage-limit errors.
// Provider selection is deferred to first Invoke call (lazy Select) so that
// provider availability is evaluated at invocation time, not at pipeline build time.
// Current design supports single-hop fallback only (primary -> one fallback).
// If all providers fail, the last error is returned. N-hop fallback can be
// added later by looping through available providers.
// Domain adapters are unaware of this fallback — same ProviderAwareInvoker interface.
//
// Known limitation: Router.Select increments the provider invocation count as a
// side effect. On the fallback path, the failed primary's count is already
// incremented. A future iteration should split Select into a read-only selection
// method and a separate RecordInvocation method to avoid double-counting.
type FallbackAdapter struct {
	router RouterSelector
	phase  string
	tier   string
	cfg    Config

	mu      sync.Mutex
	primary ProviderAwareInvoker // resolved lazily on first Invoke; re-checked if nil
}

// NewFallbackAdapter creates a FallbackAdapter that lazily resolves the primary
// provider via Router.Select on first Invoke call.
// cfg and tier are passed through so the fallback adapter can construct a
// properly configured LLMAdapter when falling back to a different provider.
func NewFallbackAdapter(router RouterSelector, phase string, cfg Config, tier string) *FallbackAdapter {
	return &FallbackAdapter{
		router: router,
		phase:  phase,
		cfg:    cfg,
		tier:   tier,
	}
}

// resolvePrimary selects the primary provider from the router.
func (f *FallbackAdapter) resolvePrimary() ProviderAwareInvoker {
	// Model name from Select is discarded: LLMAdapter resolves it again
	// internally via prov.ModelForTier(tier), so the name is not needed here.
	prov, _ := f.router.Select(f.phase, f.tier)
	if prov == nil {
		return nil
	}
	cfg := f.cfg
	cfg.Phase = f.phase
	cfg.Tier = f.tier
	return New(prov, cfg)
}

// Provider returns the primary adapter's provider (satisfies ProviderAwareInvoker).
// Triggers lazy initialization if not yet resolved. Re-resolves if primary
// was previously nil (recovery-after-cooldown semantics).
func (f *FallbackAdapter) Provider() provider.Provider {
	f.mu.Lock()
	if f.primary == nil {
		f.primary = f.resolvePrimary()
	}
	p := f.primary
	f.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.Provider()
}

// Invoke delegates to the primary invoker, resolved lazily on first call.
// On usage-limit error, logs the primary error and falls back via router.
// Each call re-checks if primary is nil under the mutex, allowing
// recovery after a provider's cooldown expires.
func (f *FallbackAdapter) Invoke(ctx context.Context, prompt string) (*provider.Result, error) {
	f.mu.Lock()
	if f.primary == nil {
		f.primary = f.resolvePrimary()
	}
	primary := f.primary
	f.mu.Unlock()
	if primary == nil {
		return nil, fmt.Errorf("no providers available for phase %q tier %q", f.phase, f.tier)
	}
	result, err := primary.Invoke(ctx, prompt)
	prov := primary.Provider()
	if err != nil && prov != nil && result != nil && prov.IsUsageLimitError(result, err) {
		primaryName := prov.Name()
		log.Printf("provider %s hit usage limit, attempting fallback: %v", primaryName, err)
		f.router.MarkUnavailable(primaryName)
		f.mu.Lock()
		f.primary = nil
		f.mu.Unlock()
		fallbackProv, _ := f.router.Select(f.phase, f.tier)
		if fallbackProv == nil {
			return result, fmt.Errorf("all providers exhausted after %s usage limit: %w", primaryName, err)
		}
		cfg := f.cfg
		cfg.Tier = f.tier
		fallback := New(fallbackProv, cfg)
		fallbackResult, fallbackErr := fallback.Invoke(ctx, prompt)
		if fallbackErr != nil {
			return fallbackResult, fmt.Errorf("fallback provider %s also failed (primary was %s): %w", fallbackProv.Name(), primaryName, fallbackErr)
		}
		log.Printf("provider fallback: %s (usage limit) -> %s (success)", primaryName, fallbackProv.Name())
		return fallbackResult, nil
	}
	return result, err
}

// InvokeInDir delegates to the primary invoker's InvokeInDir, with the same
// lazy resolution and usage-limit fallback logic as Invoke.
func (f *FallbackAdapter) InvokeInDir(ctx context.Context, prompt string, dir string) (*provider.Result, error) {
	f.mu.Lock()
	if f.primary == nil {
		f.primary = f.resolvePrimary()
	}
	primary := f.primary
	f.mu.Unlock()
	if primary == nil {
		return nil, fmt.Errorf("no providers available for phase %q tier %q", f.phase, f.tier)
	}
	result, err := primary.InvokeInDir(ctx, prompt, dir)
	prov := primary.Provider()
	if err != nil && prov != nil && result != nil && prov.IsUsageLimitError(result, err) {
		primaryName := prov.Name()
		log.Printf("provider %s hit usage limit, attempting fallback: %v", primaryName, err)
		f.router.MarkUnavailable(primaryName)
		f.mu.Lock()
		f.primary = nil
		f.mu.Unlock()
		fallbackProv, _ := f.router.Select(f.phase, f.tier)
		if fallbackProv == nil {
			return result, fmt.Errorf("all providers exhausted after %s usage limit: %w", primaryName, err)
		}
		cfg := f.cfg
		cfg.Tier = f.tier
		fallback := New(fallbackProv, cfg)
		fallbackResult, fallbackErr := fallback.InvokeInDir(ctx, prompt, dir)
		if fallbackErr != nil {
			return fallbackResult, fmt.Errorf("fallback provider %s also failed (primary was %s): %w", fallbackProv.Name(), primaryName, fallbackErr)
		}
		log.Printf("provider fallback: %s (usage limit) -> %s (success)", primaryName, fallbackProv.Name())
		return fallbackResult, nil
	}
	return result, err
}

// Compile-time interface checks.
var _ Invoker = (*FallbackAdapter)(nil)
var _ ProviderAwareInvoker = (*FallbackAdapter)(nil)
