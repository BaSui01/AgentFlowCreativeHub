import { renderHook } from '@testing-library/react';
import { describe, expect, it, beforeEach, vi } from 'vitest';

import { useAuthorization, PERMISSIONS } from './use-authorization';

const mockUseAuth = vi.fn();

vi.mock('./auth-context', () => ({
	useAuth: () => mockUseAuth(),
}));

describe('useAuthorization', () => {
	beforeEach(() => {
		mockUseAuth.mockReset();
	});

	it('grants permissions based on tenant_admin role', () => {
		mockUseAuth.mockReturnValue({ user: { roles: ['tenant_admin'] } });
		const { result } = renderHook(() => useAuthorization());
		expect(result.current.hasPermission(PERMISSIONS.WORKSPACE_WRITE)).toBe(true);
		expect(result.current.hasPermission(PERMISSIONS.MANAGE_ROLES)).toBe(true);
	});

	it('denies permissions when user lacks roles', () => {
		mockUseAuth.mockReturnValue({ user: { roles: [] } });
		const { result } = renderHook(() => useAuthorization());
		expect(result.current.permissions).toHaveLength(0);
		expect(result.current.hasPermission(PERMISSIONS.WORKSPACE_READ)).toBe(false);
	});

	it('treats super_admin as full access', () => {
		mockUseAuth.mockReturnValue({ user: { roles: ['super_admin'] } });
		const { result } = renderHook(() => useAuthorization());
		expect(result.current.permissions).toContain(PERMISSIONS.COMMAND_ADMIN);
		expect(result.current.hasPermission(PERMISSIONS.KNOWLEDGE_MANAGE)).toBe(true);
	});
});
