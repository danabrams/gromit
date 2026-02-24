package provider

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestNoopCacheAdapterLookupReturnsMiss(t *testing.T) {
	adapter := NewNoopCacheAdapter()

	entry, hit, err := adapter.Lookup(context.Background(), CacheLookupRequest{
		CacheClass: "build",
		CacheKey:   "k1",
		Config: config.TokenEfficiencyCacheConfig{
			Enabled:  true,
			TTL:      "30m",
			Capacity: 256,
		},
	})
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if hit {
		t.Fatalf("Lookup() hit = true, want false")
	}
	if entry != nil {
		t.Fatalf("Lookup() entry = %#v, want nil", entry)
	}
}

func TestNoopCacheAdapterWriteAndInvalidateAreSafeNoops(t *testing.T) {
	adapter := NewNoopCacheAdapter()

	writeErr := adapter.Write(context.Background(), CacheWriteRequest{
		CacheClass: "build",
		CacheKey:   "k1",
		Content:    "cached",
		Config: config.TokenEfficiencyCacheConfig{
			Enabled:  true,
			TTL:      "30m",
			Capacity: 256,
		},
	})
	if writeErr != nil {
		t.Fatalf("Write() error = %v, want nil", writeErr)
	}

	invalidateErr := adapter.Invalidate(context.Background(), CacheInvalidateRequest{
		CacheClass: "build",
		CacheKey:   "k1",
		Config: config.TokenEfficiencyCacheConfig{
			Enabled:  true,
			TTL:      "30m",
			Capacity: 256,
		},
	})
	if invalidateErr != nil {
		t.Fatalf("Invalidate() error = %v, want nil", invalidateErr)
	}
}

type testProviderWithoutCache struct{}

func (testProviderWithoutCache) Name() string { return "test" }
func (testProviderWithoutCache) ModelForTier(tier string) string {
	return tier
}
func (testProviderWithoutCache) Run(context.Context, string, string) (*Result, error) {
	return nil, nil
}
func (testProviderWithoutCache) StreamRun(context.Context, string, string, io.Writer, EventHandler, ToolCallHandler) (*Result, error) {
	return nil, nil
}
func (testProviderWithoutCache) RunValidation(context.Context, []string, string, string) (*Result, error) {
	return nil, nil
}
func (testProviderWithoutCache) IsUsageLimitError(*Result, error) bool { return false }
func (testProviderWithoutCache) IsValidationPassed(*Result) bool        { return false }
func (testProviderWithoutCache) IsScopeTooLarge(*Result) (bool, string) {
	return false, ""
}

func TestResolveCacheAdapterFallsBackToNoopWhenProviderLacksCapability(t *testing.T) {
	adapter := ResolveCacheAdapter(testProviderWithoutCache{})

	if SupportsProviderCache(testProviderWithoutCache{}) {
		t.Fatalf("SupportsProviderCache() = true, want false")
	}

	entry, hit, err := adapter.Lookup(context.Background(), CacheLookupRequest{
		CacheClass: "build",
		CacheKey:   "k1",
	})
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if hit {
		t.Fatalf("Lookup() hit = true, want false")
	}
	if entry != nil {
		t.Fatalf("Lookup() entry = %#v, want nil", entry)
	}
}

func TestClaudeAndCodexProvidersExposeCacheAdapterCapability(t *testing.T) {
	providers := []Provider{
		NewClaudeProvider(nil, map[string]string{TierMedium: "sonnet"}),
		NewCodexProvider("codex", nil, map[string]string{TierMedium: "gpt-5.3-codex"}),
	}

	for _, p := range providers {
		if !SupportsProviderCache(p) {
			t.Fatalf("SupportsProviderCache(%T) = false, want true", p)
		}

		adapter := ResolveCacheAdapter(p)
		if adapter == nil {
			t.Fatalf("ResolveCacheAdapter(%T) = nil, want adapter", p)
		}

		if err := adapter.Write(context.Background(), CacheWriteRequest{
			CacheClass: "build",
			CacheKey:   "k1",
			Content:    "cached",
			Config:     config.TokenEfficiencyCacheConfig{Enabled: true, TTL: "30m", Capacity: 256},
		}); err != nil {
			t.Fatalf("Write(%T) error = %v, want nil", p, err)
		}
	}
}
