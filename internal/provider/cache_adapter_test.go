package provider

import (
	"context"
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
