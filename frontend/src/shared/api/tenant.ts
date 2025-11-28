import { AXIOS_INSTANCE } from '@/api/instance';
import { extractPayload } from './helpers';

interface ListResponse<T> {
	items: T[];
	pagination?: {
		page: number;
		page_size: number;
		total: number;
		total_page: number;
	};
}

export interface TenantRoleDTO {
	id: string;
	name: string;
	code: string;
	description?: string;
	isSystem?: boolean;
	isDefault?: boolean;
	priority?: number;
	permissionIds: string[];
	createdAt?: string;
	updatedAt?: string;
}

export interface TenantPermissionDTO {
	id: string;
	code: string;
	name: string;
	category?: string;
	resource: string;
	action: string;
	description?: string;
}

export interface PermissionLocaleDTO {
	name: string;
	description?: string;
}

export interface PermissionCatalogItemDTO {
	code: string;
	category: string;
	resource: string;
	action: string;
	name: string;
	description?: string;
	locales?: Record<string, PermissionLocaleDTO>;
}

export interface PermissionCatalogDTO {
	version: string;
	items: PermissionCatalogItemDTO[];
	categoryLabels: Record<string, Record<string, string>>;
	categoryOrder: string[];
}

export interface TenantUserDTO {
	id: string;
	tenantId: string;
	email: string;
	username: string;
	status: string;
	createdAt?: string;
	updatedAt?: string;
}

export interface AuditLogDTO {
	id: string;
	action: string;
	resource: string;
	resourceId?: string;
	details?: Record<string, unknown>;
	userId?: string;
	status?: string;
	createdAt: string;
}

export interface UpsertRolePayload {
	id?: string;
	name: string;
	description?: string;
	permissionIds: string[];
}

const unwrapList = <T>(payload: { data?: ListResponse<T> } | ListResponse<T>): T[] => {
	const list = extractPayload<ListResponse<T>>(payload);
	return Array.isArray(list?.items) ? list.items : [];
};

export const TenantAPI = {
	async listRoles(tenantId: string): Promise<TenantRoleDTO[]> {
		const resp = await AXIOS_INSTANCE.get(`/api/tenants/${tenantId}/roles`);
		return unwrapList<TenantRoleDTO>(resp.data);
	},
	async createRole(tenantId: string, payload: UpsertRolePayload): Promise<TenantRoleDTO> {
		const resp = await AXIOS_INSTANCE.post(`/api/tenants/${tenantId}/roles`, payload);
		return extractPayload<TenantRoleDTO>(resp.data);
	},
	async updateRole(tenantId: string, payload: UpsertRolePayload & { id: string }): Promise<TenantRoleDTO> {
		const resp = await AXIOS_INSTANCE.put(`/api/tenants/${tenantId}/roles`, payload);
		return extractPayload<TenantRoleDTO>(resp.data);
	},
	async deleteRole(tenantId: string, roleId: string): Promise<void> {
		await AXIOS_INSTANCE.delete(`/api/tenants/${tenantId}/roles/${roleId}`);
	},
	async listPermissions(): Promise<TenantPermissionDTO[]> {
		const resp = await AXIOS_INSTANCE.get('/api/tenant/permissions');
		return unwrapList<TenantPermissionDTO>(resp.data);
	},
	async getPermissionCatalog(): Promise<PermissionCatalogDTO> {
		const resp = await AXIOS_INSTANCE.get('/api/tenant/permissions/catalog');
		return resp.data;
	},
	async listUsers(tenantId: string): Promise<TenantUserDTO[]> {
		const resp = await AXIOS_INSTANCE.get(`/api/tenants/${tenantId}/users`);
		return unwrapList<TenantUserDTO>(resp.data);
	},
	async replaceUserRoles(tenantId: string, userId: string, roleIds: string[]): Promise<void> {
		await AXIOS_INSTANCE.put(`/api/tenants/${tenantId}/users/${userId}/roles`, { roleIds });
	},
	async getUserRoles(tenantId: string, userId: string): Promise<string[]> {
		const resp = await AXIOS_INSTANCE.get(`/api/tenants/${tenantId}/users/${userId}/roles`);
		return unwrapList<string>(resp.data);
	},
	async listAuditLogs(limit = 10): Promise<AuditLogDTO[]> {
		const resp = await AXIOS_INSTANCE.get('/api/tenant/audit-logs', { params: { limit } });
		return unwrapList<AuditLogDTO>(resp.data);
	},
};

export type { UpsertRolePayload };
