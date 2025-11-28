import { AXIOS_INSTANCE } from '@/api/instance';
import {
	WorkspaceFileDetailDTO,
	WorkspaceNodeDTO,
	WorkspaceStagingFileDTO,
	AttachContextPayload,
	ExecuteCommandPayload,
	ExecuteCommandResponse,
	CommandRequestDTO,
	CommandListResult,
} from './types';

export const WorkspaceAPI = {
  async fetchTree(): Promise<WorkspaceNodeDTO[]> {
    const resp = await AXIOS_INSTANCE.get('/api/files/tree', { params: { depth: 2 } });
    return resp.data?.data?.nodes ?? [];
  },

  async createFolder(payload: { name: string; parentId?: string | null; category?: string }): Promise<WorkspaceNodeDTO> {
    const resp = await AXIOS_INSTANCE.post('/api/workspace/folders', payload);
    return resp.data?.data;
  },

  async renameNode(id: string, name: string): Promise<WorkspaceNodeDTO> {
    const resp = await AXIOS_INSTANCE.patch(`/api/workspace/nodes/${id}`, { name });
    return resp.data?.data;
  },

  async deleteNode(id: string): Promise<void> {
    await AXIOS_INSTANCE.delete(`/api/workspace/nodes/${id}`);
  },

  async getFile(id: string): Promise<WorkspaceFileDetailDTO> {
    const resp = await AXIOS_INSTANCE.get('/api/files/content', { params: { nodeId: id } });
    return resp.data?.data;
  },

  async saveFile(payload: { nodeId: string; content: string; summary?: string; versionId?: string }): Promise<WorkspaceFileDetailDTO> {
    const headers = payload.versionId ? { 'If-Match': payload.versionId } : undefined;
    const resp = await AXIOS_INSTANCE.post('/api/files', {
      nodeId: payload.nodeId,
      content: payload.content,
      summary: payload.summary,
    }, { headers });
    return resp.data?.data;
  },

  async listStaging(status?: string): Promise<WorkspaceStagingFileDTO[]> {
    const resp = await AXIOS_INSTANCE.get('/api/workspace/staging', { params: { status } });
    return resp.data?.data?.items ?? [];
  },

	async createStaging(payload: { fileType: string; content: string; summary?: string; titleHint?: string; manualFolder?: string; requiresSecondary?: boolean }): Promise<WorkspaceStagingFileDTO> {
    const resp = await AXIOS_INSTANCE.post('/api/workspace/staging', payload);
    return resp.data?.data;
  },

	async reviewStaging(id: string, payload: { action: 'approve' | 'reject' | 'request_changes'; reviewToken: string; reason?: string }): Promise<WorkspaceStagingFileDTO> {
		const resp = await AXIOS_INSTANCE.post(`/api/workspace/staging/${id}/review`, payload);
		return resp.data?.data;
	},

  async attachContext(payload: AttachContextPayload): Promise<string> {
    const resp = await AXIOS_INSTANCE.post('/api/workspace/context-links', payload);
    return resp.data?.data?.sessionId;
  },

  async executeCommand(payload: ExecuteCommandPayload): Promise<ExecuteCommandResponse> {
    const resp = await AXIOS_INSTANCE.post('/api/commands/execute', payload);
    return resp.data?.data;
  },

  async getCommandById(id: string): Promise<CommandRequestDTO | undefined> {
    const resp = await AXIOS_INSTANCE.get(`/api/commands/${id}`);
    return resp.data?.data;
  },

	async listCommands(params: { status?: string; agentId?: string; page?: number; pageSize?: number }): Promise<CommandListResult> {
		const resp = await AXIOS_INSTANCE.get('/api/commands', {
			params: {
				status: params.status,
				agentId: params.agentId,
				page: params.page,
				pageSize: params.pageSize,
			},
		});
		const items: CommandRequestDTO[] = resp.data?.items ?? [];
		const pagination = resp.data?.pagination ?? {};
		const page = typeof pagination.page === 'number' ? pagination.page : params.page ?? 1;
		const pageSize = typeof pagination.page_size === 'number' ? pagination.page_size : params.pageSize ?? 20;
		const total = typeof pagination.total === 'number' ? pagination.total : items.length;
		return { items, total, page, pageSize };
	},
};
