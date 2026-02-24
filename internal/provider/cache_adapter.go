package provider

import (
	"context"

	"github.com/danabrams/gromit/internal/config"
)

type CacheEntry struct {
	Content string
}

type CacheLookupRequest struct {
	CacheClass string
	CacheKey   string
	Config     config.TokenEfficiencyCacheConfig
}

type CacheAdapter interface {
	Lookup(ctx context.Context, req CacheLookupRequest) (*CacheEntry, bool, error)
}

type noopCacheAdapter struct{}

func NewNoopCacheAdapter() CacheAdapter {
	return noopCacheAdapter{}
}

func (noopCacheAdapter) Lookup(context.Context, CacheLookupRequest) (*CacheEntry, bool, error) {
	return nil, false, nil
}
