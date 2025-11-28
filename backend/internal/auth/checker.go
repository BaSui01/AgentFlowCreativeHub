package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backend/internal/tenant"
)

// DatabasePermissionChecker implements PermissionChecker using the database.
type DatabasePermissionChecker struct {
	roleService tenant.RoleService
	cache       map[string]cacheEntry
	mu          sync.RWMutex
	cacheTTL    time.Duration
}

type cacheEntry struct {
	perms   []*tenant.Permission
	expires time.Time
}

// NewDatabasePermissionChecker creates a new DatabasePermissionChecker.
func NewDatabasePermissionChecker(roleService tenant.RoleService) *DatabasePermissionChecker {
	return &DatabasePermissionChecker{
		roleService: roleService,
		cache:       make(map[string]cacheEntry),
		cacheTTL:    5 * time.Minute,
	}
}

// HasPermission checks if the user has the required permission.
func (c *DatabasePermissionChecker) HasPermission(tc tenant.TenantContext, resource, action string) (bool, error) {
	// Create a context with the tenant context
	ctx := tenant.WithTenantContext(context.Background(), tc)
	key := cacheKey(tc.TenantID, tc.UserID)
	if perms, ok := c.getCached(key); ok {
		if matchPermission(perms, resource, action) {
			return true, nil
		}
	}

	// Get all permissions for the user
	perms, err := c.roleService.GetUserPermissions(ctx, tc.UserID)
	if err != nil {
		return false, fmt.Errorf("failed to get user permissions: %w", err)
	}
	c.setCached(key, perms)

	// Check if any permission matches the requested resource and action
	return matchPermission(perms, resource, action), nil
}

func (c *DatabasePermissionChecker) getCached(key string) ([]*tenant.Permission, bool) {
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.perms, true
}

func (c *DatabasePermissionChecker) setCached(key string, perms []*tenant.Permission) {
	c.mu.Lock()
	c.cache[key] = cacheEntry{perms: perms, expires: time.Now().Add(c.cacheTTL)}
	c.mu.Unlock()
}

// Invalidate 清理指定用户的权限缓存
func (c *DatabasePermissionChecker) Invalidate(tenantID, userID string) {
	key := cacheKey(tenantID, userID)
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

func cacheKey(tenantID, userID string) string {
	return tenantID + ":" + userID
}

func matchPermission(perms []*tenant.Permission, resource, action string) bool {
	for _, p := range perms {
		resourceMatch := p.Resource == "*" || p.Resource == resource
		actionMatch := p.Action == "*" || p.Action == action
		if resourceMatch && actionMatch {
			return true
		}
	}
	return false
}
