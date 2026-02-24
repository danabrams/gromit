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

type CacheWriteRequest struct {
	CacheClass string
	CacheKey   string
	Content    string
	Config     config.TokenEfficiencyCacheConfig
}

type CacheInvalidateRequest struct {
	CacheClass string
	CacheKey   string
	Config     config.TokenEfficiencyCacheConfig
}

type CacheAdapter interface {
	Lookup(ctx context.Context, req CacheLookupRequest) (*CacheEntry, bool, error)
	Write(ctx context.Context, req CacheWriteRequest) error
	Invalidate(ctx context.Context, req CacheInvalidateRequest) error
}

type noopCacheAdapter struct{}

func NewNoopCacheAdapter() CacheAdapter {
	return noopCacheAdapter{}
}

func (noopCacheAdapter) Lookup(context.Context, CacheLookupRequest) (*CacheEntry, bool, error) {
	return nil, false, nil
}

func (noopCacheAdapter) Write(context.Context, CacheWriteRequest) error {
	return nil
}

func (noopCacheAdapter) Invalidate(context.Context, CacheInvalidateRequest) error {
	return nil
}
