package tenant

import (
	"context"
	"sync"
	"time"
)

// TenantConfigCache defines a cache abstraction for TenantConfig. The concrete
// implementation in this module is an in-memory, per-process cache with TTL,
// which can later be replaced or backed by Redis without changing callsites.
type TenantConfigCache interface {
	Get(ctx context.Context) (*TenantConfig, bool)
	Set(ctx context.Context, cfg *TenantConfig)
	Invalidate(tenantID string)
}

type cacheEntry struct {
	value     *TenantConfig
	expiresAt time.Time
}

type inMemoryTenantConfigCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

// NewInMemoryTenantConfigCache creates a TenantConfig cache backed by a
// simple in-memory map with the given TTL.
func NewInMemoryTenantConfigCache(ttl time.Duration) TenantConfigCache {
	return &inMemoryTenantConfigCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

func (c *inMemoryTenantConfigCache) tenantIDFromCtx(ctx context.Context) (string, bool) {
	tc, ok := FromContext(ctx)
	if !ok || tc.TenantID == "" {
		return "", false
	}
	return tc.TenantID, true
}

func (c *inMemoryTenantConfigCache) Get(ctx context.Context) (*TenantConfig, bool) {
	tenantID, ok := c.tenantIDFromCtx(ctx)
	if !ok {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.entries[tenantID]
	if !found || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (c *inMemoryTenantConfigCache) Set(ctx context.Context, cfg *TenantConfig) {
	tenantID, ok := c.tenantIDFromCtx(ctx)
	if !ok {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[tenantID] = cacheEntry{
		value:     cfg,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *inMemoryTenantConfigCache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, tenantID)
}
