package auth

import (
	"context"
	"fmt"

	"backend/internal/tenant"
)

// DatabasePermissionChecker implements PermissionChecker using the database.
type DatabasePermissionChecker struct {
	roleService tenant.RoleService
}

// NewDatabasePermissionChecker creates a new DatabasePermissionChecker.
func NewDatabasePermissionChecker(roleService tenant.RoleService) *DatabasePermissionChecker {
	return &DatabasePermissionChecker{
		roleService: roleService,
	}
}

// HasPermission checks if the user has the required permission.
func (c *DatabasePermissionChecker) HasPermission(tc tenant.TenantContext, resource, action string) (bool, error) {
	// Create a context with the tenant context
	ctx := tenant.WithTenantContext(context.Background(), tc)

	// Get all permissions for the user
	perms, err := c.roleService.GetUserPermissions(ctx, tc.UserID)
	if err != nil {
		return false, fmt.Errorf("failed to get user permissions: %w", err)
	}

	// Check if any permission matches the requested resource and action
	for _, p := range perms {
		// Support wildcard matching
		resourceMatch := p.Resource == "*" || p.Resource == resource
		actionMatch := p.Action == "*" || p.Action == action

		if resourceMatch && actionMatch {
			return true, nil
		}
	}

	return false, nil
}
