import { render, screen } from '@testing-library/react';
import { describe, expect, it, beforeEach, vi } from 'vitest';

import { PermissionGuard } from './PermissionGuard';

const mockAuthorization = vi.fn(() => ({
	hasAnyRole: () => true,
	hasAnyPermission: () => true,
}));

vi.mock('@/features/auth/model/use-authorization', () => ({
	useAuthorization: () => mockAuthorization(),
}));

describe('PermissionGuard', () => {
	beforeEach(() => {
		mockAuthorization.mockReset();
	});

	it('renders children when allowed', () => {
		mockAuthorization.mockReturnValue({ hasAnyRole: () => true, hasAnyPermission: () => true });
		render(
			<PermissionGuard requiredPermissions={['perm']}> 
				<div>allowed</div>
			</PermissionGuard>,
		);
		expect(screen.getByText('allowed')).toBeInTheDocument();
	});

	it('renders fallback when denied', () => {
		mockAuthorization.mockReturnValue({ hasAnyRole: () => false, hasAnyPermission: () => false });
		render(
			<PermissionGuard requiredPermissions={['perm']} fallback={<div>denied</div>}>
				<div>allowed</div>
			</PermissionGuard>,
		);
		expect(screen.getByText('denied')).toBeInTheDocument();
	});
});
