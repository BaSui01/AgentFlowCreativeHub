import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import type { TenantUserDTO } from '@/shared/api/tenant';
import { TenantAPI } from '@/shared/api/tenant';

export const USER_ROLE_QUERY_KEY = ['tenant-user-roles'];

export const useTenantUserRoles = (tenantId?: string, users?: TenantUserDTO[]) => {
	const userIdsKey = useMemo(() => {
		if (!users || users.length === 0) {
			return '';
		}
		return users
			.map((user) => user.id)
			.sort()
			.join(',');
	}, [users]);

	return useQuery<Record<string, string[]>>({
		queryKey: [...USER_ROLE_QUERY_KEY, tenantId, userIdsKey],
		enabled: Boolean(tenantId && userIdsKey),
		queryFn: async () => {
			if (!tenantId || !users) {
				return {};
			}
			const entries = await Promise.all(
				users.map(async (user) => {
					const roles = await TenantAPI.getUserRoles(tenantId, user.id);
					return [user.id, roles] as const;
				}),
			);
			return Object.fromEntries(entries);
		},
	});
};
