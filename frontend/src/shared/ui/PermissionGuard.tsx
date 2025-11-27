import React from 'react';

import { useAuthorization } from '@/features/auth/model/use-authorization';

interface PermissionGuardProps {
	requiredRoles?: string[];
	requiredPermissions?: string[];
	fallback?: React.ReactNode;
	children: React.ReactNode;
}

export const PermissionGuard: React.FC<PermissionGuardProps> = ({
	children,
	requiredRoles,
	requiredPermissions,
	fallback = null,
}) => {
	const { hasAnyRole, hasAnyPermission } = useAuthorization();
	const roleAllowed = requiredRoles ? hasAnyRole(requiredRoles) : true;
	const permissionAllowed = requiredPermissions ? hasAnyPermission(requiredPermissions) : true;
	if (!roleAllowed || !permissionAllowed) {
		return <>{fallback}</>;
	}
	return <>{children}</>;
};
