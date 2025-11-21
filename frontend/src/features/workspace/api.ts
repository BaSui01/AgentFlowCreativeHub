import { AXIOS_INSTANCE } from '@/api/instance';
import {
  WorkspaceFileDetailDTO,
  WorkspaceNodeDTO,
  WorkspaceStagingFileDTO,
  AttachContextPayload,
} from './types';

export const WorkspaceAPI = {
  async fetchTree(): Promise<WorkspaceNodeDTO[]> {
    const resp = await AXIOS_INSTANCE.get('/api/workspace/tree');
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
    const resp = await AXIOS_INSTANCE.get(`/api/workspace/files/${id}`);
    return resp.data?.data;
  },

  async updateFile(id: string, payload: { content: string; summary?: string }): Promise<WorkspaceFileDetailDTO> {
    const resp = await AXIOS_INSTANCE.put(`/api/workspace/files/${id}`, payload);
    return resp.data?.data;
  },

  async listStaging(status?: string): Promise<WorkspaceStagingFileDTO[]> {
    const resp = await AXIOS_INSTANCE.get('/api/workspace/staging', { params: { status } });
    return resp.data?.data?.items ?? [];
  },

  async createStaging(payload: { fileType: string; content: string; summary?: string; titleHint?: string; manualFolder?: string }): Promise<WorkspaceStagingFileDTO> {
    const resp = await AXIOS_INSTANCE.post('/api/workspace/staging', payload);
    return resp.data?.data;
  },

  async approveStaging(id: string): Promise<Record<string, unknown>> {
    const resp = await AXIOS_INSTANCE.post(`/api/workspace/staging/${id}/approve`);
    return resp.data?.data ?? {};
  },

  async rejectStaging(id: string, reason: string): Promise<void> {
    await AXIOS_INSTANCE.post(`/api/workspace/staging/${id}/reject`, { reason });
  },

  async attachContext(payload: AttachContextPayload): Promise<string> {
    const resp = await AXIOS_INSTANCE.post('/api/workspace/context-links', payload);
    return resp.data?.data?.sessionId;
  },
};
