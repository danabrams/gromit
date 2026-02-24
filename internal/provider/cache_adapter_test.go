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
