import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { LoginPage } from './LoginPage';

const loginMock = vi.fn();

const hoistedAPI = vi.hoisted(() => ({
	getHealth: vi.fn().mockResolvedValue({ status: 'healthy' }),
	getReady: vi.fn().mockResolvedValue({ status: 'ready' }),
}));

vi.mock('@/features/auth/model/auth-context', () => {
	return {
		useAuth: () => authStub,
	};
});

vi.mock('@/shared/api', () => ({
	PublicAPI: hoistedAPI,
}));

type AuthStub = {
	login: typeof loginMock;
	isLoading: boolean;
	error?: string;
	isAuthenticated: boolean;
};

let authStub: AuthStub = {
	login: loginMock,
	isLoading: false,
	isAuthenticated: false,
};

describe('LoginPage', () => {
	beforeEach(() => {
		loginMock.mockResolvedValue(undefined);
		hoistedAPI.getHealth.mockResolvedValue({ status: 'healthy' });
		hoistedAPI.getReady.mockResolvedValue({ status: 'ready' });
		authStub = {
			login: loginMock,
			isLoading: false,
			error: undefined,
			isAuthenticated: false,
		};
		vi.clearAllMocks();
	});

	it('submits email and password', async () => {
		renderWithProviders();

		await userEvent.type(screen.getByPlaceholderText('name@example.com'), 'user@example.com');
		await userEvent.type(screen.getByPlaceholderText('请输入密码'), 'secret123');
	await userEvent.click(screen.getByRole('button', { name: /登\s*录/ }));

		expect(loginMock).toHaveBeenCalledWith({ email: 'user@example.com', password: 'secret123' });
	});

	it('renders error message from auth state', () => {
		authStub.error = '认证失败';
		renderWithProviders();

		expect(screen.getByText('认证失败')).toBeInTheDocument();
	});
});

const renderWithProviders = () => {
	const client = new QueryClient();
	return render(
		<QueryClientProvider client={client}>
			<MemoryRouter>
				<LoginPage />
			</MemoryRouter>
		</QueryClientProvider>,
	);
};

