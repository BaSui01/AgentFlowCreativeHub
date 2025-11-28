import { AXIOS_INSTANCE } from '@/api/instance';
import type {
	AuthLoginRequest,
	AuthLoginResponse,
	AuthRefreshRequest,
	AuthTokenPair,
	PostApiAuthLogoutBody,
} from '@/api/generated/model';
import { extractPayload } from './helpers';

export const AuthAPI = {
	async login(payload: AuthLoginRequest): Promise<AuthLoginResponse> {
		const resp = await AXIOS_INSTANCE.post('/api/auth/login', payload);
		return extractPayload<AuthLoginResponse>(resp.data);
	},
	async logout(payload?: PostApiAuthLogoutBody): Promise<void> {
		await AXIOS_INSTANCE.post('/api/auth/logout', payload ?? {});
	},
	async refresh(payload: AuthRefreshRequest): Promise<AuthTokenPair> {
		const resp = await AXIOS_INSTANCE.post('/api/auth/refresh', payload);
		return extractPayload<AuthTokenPair>(resp.data);
	},
};

export type { AuthLoginRequest, AuthLoginResponse, AuthTokenPair };
