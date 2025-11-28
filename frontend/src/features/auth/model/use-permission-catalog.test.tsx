import React from 'react';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi, beforeEach } from 'vitest';

import { usePermissionCatalog, normalizePermissionCatalog } from './use-authorization';
import { TenantAPI } from '@/shared/api/tenant';

vi.mock('@/shared/api/tenant', () => ({
	TenantAPI: {
		getPermissionCatalog: vi.fn(),
	},
}));

const createWrapper = () => {
	const queryClient = new QueryClient();
	return ({ children }: { children: React.ReactNode }) => (
		<QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
	);
};

describe('usePermissionCatalog', () => {
	beforeEach(() => {
		vi.resetAllMocks();
	});

	it('fetches permission catalog via TenantAPI', async () => {
		const mockCatalog = {
			version: 'v1',
			items: [{ code: 'workspace:read', category: 'workspace', resource: 'workspace', action: 'read', name: '查看文件' }],
			categoryLabels: {},
			categoryOrder: [],
		};
		(TenantAPI.getPermissionCatalog as unknown as vi.Mock).mockResolvedValue(mockCatalog);
		const wrapper = createWrapper();
		const { result } = renderHook(() => usePermissionCatalog(), { wrapper });
		await waitFor(() => {
			expect(result.current.data?.items[0].code).toBe('workspace:read');
		});
	});

	it('normalizes locales with fallback', () => {
		const normalized = normalizePermissionCatalog({
			version: 'v1',
			items: [
				{
					code: 'tenant:manage_roles',
					category: 'tenant',
					resource: 'tenant',
					action: 'manage_roles',
					name: '管理角色',
					locales: {
						'zh-CN': { name: '管理角色', description: '中文描述' },
					},
				},
			],
			categoryLabels: {},
			categoryOrder: [],
		});
		const item = normalized?.items[0];
		expect(item?.locales?.['en-US']).toBeTruthy();
		expect(item?.locales?.['en-US']?.name).toBe('管理角色');
	});
});
