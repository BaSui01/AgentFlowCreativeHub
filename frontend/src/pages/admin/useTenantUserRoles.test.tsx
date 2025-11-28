import React from 'react';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi, beforeEach } from 'vitest';

import { useTenantUserRoles } from './useTenantUserRoles';
import { TenantAPI } from '@/shared/api/tenant';

vi.mock('@/shared/api/tenant', () => ({
	TenantAPI: {
		getUserRoles: vi.fn(),
	},
}));

const createWrapper = () => {
	const client = new QueryClient();
	return ({ children }: { children: React.ReactNode }) => (
		<QueryClientProvider client={client}>{children}</QueryClientProvider>
	);
};

describe('useTenantUserRoles', () => {
	beforeEach(() => {
		vi.resetAllMocks();
	});

	it('fetches user roles when tenant and users provided', async () => {
		const mockGet = TenantAPI.getUserRoles as unknown as vi.Mock;
		mockGet.mockResolvedValueOnce(['role-admin']);
		mockGet.mockResolvedValueOnce(['role-editor']);
		const wrapper = createWrapper();
		const { result } = renderHook(() =>
			useTenantUserRoles('tenant-1', [
				{ id: 'user-1', tenantId: 'tenant-1', email: 'a@x.com', username: 'A', status: 'active' },
				{ id: 'user-2', tenantId: 'tenant-1', email: 'b@x.com', username: 'B', status: 'active' },
			]),
			{ wrapper },
		);
		await waitFor(() => {
			expect(result.current.data?.['user-1']).toEqual(['role-admin']);
			expect(result.current.data?.['user-2']).toEqual(['role-editor']);
		});
	});

	it('skips query when users list empty', async () => {
		const mockGet = TenantAPI.getUserRoles as unknown as vi.Mock;
		const wrapper = createWrapper();
		renderHook(() => useTenantUserRoles('tenant-1', []), { wrapper });
		await waitFor(() => {
			expect(mockGet).not.toHaveBeenCalled();
		});
	});
});
