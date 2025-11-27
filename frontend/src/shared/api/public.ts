import { AXIOS_INSTANCE } from '@/api/instance';
import type { ApiHealthResponse, ApiReadinessResponse } from '@/api/generated/model';
import { extractPayload } from './helpers';

export const PublicAPI = {
	async getHealth(): Promise<ApiHealthResponse> {
		const resp = await AXIOS_INSTANCE.get('/health');
		return extractPayload<ApiHealthResponse>(resp.data);
	},
	async getReady(): Promise<ApiReadinessResponse> {
		const resp = await AXIOS_INSTANCE.get('/ready');
		return extractPayload<ApiReadinessResponse>(resp.data);
	},
};
