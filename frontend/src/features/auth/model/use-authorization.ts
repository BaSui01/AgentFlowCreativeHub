import { useCallback, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { useAuth } from './auth-context';
import type { PermissionCatalogDTO, PermissionLocaleDTO } from '@/shared/api/tenant';
import { TenantAPI } from '@/shared/api/tenant';

export const PERMISSIONS = {
	MANAGE_ROLES: 'tenant:manage_roles',
	MANAGE_USERS: 'tenant:manage_users',
	WORKSPACE_READ: 'workspace:read',
	WORKSPACE_WRITE: 'workspace:write',
	WORKSPACE_REVIEW: 'workspace:review',
	WORKFLOW_CREATE: 'workflow:create',
	WORKFLOW_APPROVE: 'workflow:approve',
	WORKFLOW_EXECUTE: 'workflow:execute',
	KNOWLEDGE_MANAGE: 'kb:manage',
	COMMAND_EXECUTE: 'commands:execute',
	COMMAND_ADMIN: 'commands:admin',
} as const;

const ALL_PERMISSIONS = Object.values(PERMISSIONS);

const ROLE_PERMISSION_MAP: Record<string, string[]> = {
	super_admin: ['*'],
	system_admin: ['*'],
	tenant_admin: ALL_PERMISSIONS,
	workspace_admin: [
		PERMISSIONS.WORKSPACE_READ,
		PERMISSIONS.WORKSPACE_WRITE,
		PERMISSIONS.WORKSPACE_REVIEW,
		PERMISSIONS.COMMAND_EXECUTE,
		PERMISSIONS.COMMAND_ADMIN,
	],
	reviewer: [PERMISSIONS.WORKSPACE_READ, PERMISSIONS.WORKSPACE_REVIEW],
	editor: [PERMISSIONS.WORKSPACE_READ, PERMISSIONS.WORKSPACE_WRITE, PERMISSIONS.COMMAND_EXECUTE],
	viewer: [PERMISSIONS.WORKSPACE_READ],
};

const normalize = (value?: string) => value?.trim().toLowerCase();

export const PERMISSION_CATALOG_QUERY_KEY = ['permission-catalog'];

export const normalizePermissionCatalog = (catalog?: PermissionCatalogDTO): PermissionCatalogDTO | undefined => {
	if (!catalog) {
		return catalog;
	}
	const normalizedItems = catalog.items.map((item) => {
		const baseLocale: PermissionLocaleDTO = { name: item.name, description: item.description };
		const locales = {
			...item.locales,
			'zh-CN': item.locales?.['zh-CN'] ?? baseLocale,
			'en-US': item.locales?.['en-US'] ?? item.locales?.['zh-CN'] ?? baseLocale,
		};
		return { ...item, locales };
	});
	return { ...catalog, items: normalizedItems };
};

export const usePermissionCatalog = () =>
	useQuery<PermissionCatalogDTO>({
		queryKey: PERMISSION_CATALOG_QUERY_KEY,
		queryFn: () => TenantAPI.getPermissionCatalog(),
		staleTime: 5 * 60 * 1000,
		select: normalizePermissionCatalog,
	});

export const useAuthorization = () => {
	const { user } = useAuth();
	const roleList = useMemo(() => {
		return (user?.roles ?? [])
			.map(normalize)
			.filter((role): role is string => Boolean(role));
	}, [user?.roles]);
	const permissionList = useMemo(() => {
		const userPermissions = (user as unknown as { permissions?: string[] } | undefined)?.permissions;
		if (Array.isArray(userPermissions) && userPermissions.length) {
			return Array.from(new Set(userPermissions));
		}
		if (!roleList.length) {
			return [];
		}
		if (roleList.some((role) => ROLE_PERMISSION_MAP[role]?.includes('*'))) {
			return ALL_PERMISSIONS;
		}
		const merged = new Set<string>();
		roleList.forEach((role) => {
			const perms = ROLE_PERMISSION_MAP[role];
			perms?.forEach((perm) => merged.add(perm));
		});
		return Array.from(merged);
	}, [roleList, user]);
	const permissionSet = useMemo(() => new Set(permissionList), [permissionList]);

	const hasRole = useCallback((role?: string) => {
		const normalized = normalize(role);
		return normalized ? roleList.includes(normalized) : false;
	}, [roleList]);

	const hasAnyRole = useCallback((roles?: string[]) => {
		if (!roles || roles.length === 0) {
			return roleList.length > 0;
		}
		return roles.some((role) => hasRole(role));
	}, [hasRole, roleList]);

	const hasPermission = useCallback((permission?: string) => {
		if (!permission) {
			return false;
		}
		return permissionSet.has(permission);
	}, [permissionSet]);

	const hasAnyPermission = useCallback((permissions?: string[]) => {
		if (!permissions || permissions.length === 0) {
			return permissionSet.size > 0;
		}
		return permissions.some((perm) => hasPermission(perm));
	}, [hasPermission, permissionSet.size]);

	return {
		roles: roleList,
		permissions: permissionList,
		hasRole,
		hasAnyRole,
		hasPermission,
		hasAnyPermission,
	};
};
