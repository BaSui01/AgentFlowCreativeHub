import React, { useMemo, useState } from 'react';
import { ReloadOutlined } from '@ant-design/icons';
import {
	Alert,
	Button,
	Card,
	Form,
	Input,
	List,
	Modal,
	Select,
	Space,
	Table,
	Tabs,
	Tag,
	Typography,
	message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useAuth } from '@/features/auth/model/auth-context';
import { PERMISSIONS, useAuthorization, usePermissionCatalog } from '@/features/auth/model/use-authorization';
import type { TenantPermissionDTO, TenantRoleDTO, TenantUserDTO, UpsertRolePayload, PermissionCatalogItemDTO, AuditLogDTO } from '@/shared/api/tenant';
import { TenantAPI } from '@/shared/api/tenant';
import { useTenantUserRoles, USER_ROLE_QUERY_KEY } from './useTenantUserRoles';

const { Title, Paragraph } = Typography;

type EnrichedPermission = TenantPermissionDTO & {
	displayName: string;
	displayDescription?: string;
	catalogCategory: string;
};

const getPreferredLocale = () => {
	if (typeof navigator === 'undefined' || !navigator.language) {
		return 'zh-CN';
	}
	return navigator.language.toLowerCase().includes('en') ? 'en-US' : 'zh-CN';
};

interface PermissionChecklistProps {
	grouped: Record<string, EnrichedPermission[]>;
	orderedKeys: string[];
	value?: string[];
	onChange?: (value: string[]) => void;
	categoryLabels: Record<string, string>;
}

const PermissionChecklist: React.FC<PermissionChecklistProps> = ({ grouped, orderedKeys, value = [], onChange, categoryLabels }) => {
	const handleToggle = (permissionId: string, checked: boolean) => {
		const next = checked ? [...value, permissionId] : value.filter((id) => id !== permissionId);
		onChange?.(Array.from(new Set(next)));
	};

	const categories = orderedKeys.length
		? orderedKeys.filter((key) => (grouped[key] ?? []).length > 0)
		: Object.keys(grouped).sort();

	return (
		<Space direction="vertical" style={{ width: '100%' }}>
			{categories.map((category) => (
				<div key={category}>
					<Typography.Text strong style={{ display: 'block', marginBottom: 4 }}>
						{categoryLabels[category] ?? category}
					</Typography.Text>
					<Space direction="vertical" style={{ width: '100%' }}>
						{(grouped[category] ?? []).map((perm) => (
							<label key={perm.id} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
								<input
									type="checkbox"
									checked={value.includes(perm.id)}
									onChange={(event) => handleToggle(perm.id, event.target.checked)}
								/>
								<span style={{ flex: 1 }}>
									<Typography.Text>
										{perm.displayName}（{perm.code}）
									</Typography.Text>
									{perm.displayDescription && (
										<Typography.Text type="secondary" style={{ marginLeft: 8 }}>
											{perm.displayDescription}
										</Typography.Text>
									)}
								</span>
							</label>
						))}
					</Space>
				</div>
			))}
		</Space>
	);
};
						{permissions.map((perm) => (
							<label key={perm.id} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
								<input
									type="checkbox"
									checked={value.includes(perm.id)}
									onChange={(event) => handleToggle(perm.id, event.target.checked)}
								/>
								<span>
									{perm.name}（{perm.code}）
									{perm.description && (
										<Typography.Text type="secondary" style={{ marginLeft: 8 }}>
											{perm.description}
										</Typography.Text>
									)}
								</span>
							</label>
						))}
					</Space>
				</div>
			))}
		</Space>
	);
};

export const RoleManagementPage: React.FC = () => {
	const { user } = useAuth();
	const { hasPermission, roles: currentRoles, permissions: currentPermissions } = useAuthorization();
	const tenantId = user?.tenant_id;
	const queryClient = useQueryClient();
	const preferredLocale = useMemo(() => getPreferredLocale(), []);
	const rolesQuery = useQuery({
		queryKey: ['tenant-roles', tenantId],
		queryFn: () => TenantAPI.listRoles(tenantId ?? ''),
		enabled: Boolean(tenantId),
	});
	const permissionsQuery = useQuery({
		queryKey: ['tenant-permissions'],
		queryFn: () => TenantAPI.listPermissions(),
	});
	const permissionCatalogQuery = usePermissionCatalog();
	const usersQuery = useQuery({
		queryKey: ['tenant-users', tenantId],
		queryFn: () => TenantAPI.listUsers(tenantId ?? ''),
		enabled: Boolean(tenantId),
	});
	const auditLogsQuery = useQuery({
		queryKey: ['tenant-audit-logs', tenantId],
		queryFn: () => TenantAPI.listAuditLogs(10),
		enabled: Boolean(tenantId),
	});
	const [roleForm] = Form.useForm<UpsertRolePayload>();
	const [editingRole, setEditingRole] = useState<TenantRoleDTO | null>(null);
	const [roleModalOpen, setRoleModalOpen] = useState(false);
	const [assignTarget, setAssignTarget] = useState<TenantUserDTO | null>(null);
	const [assignRoles, setAssignRoles] = useState<string[]>([]);
	const [permissionSearch, setPermissionSearch] = useState('');
	const auditLogs = auditLogsQuery.data ?? [];

	const userRoleMapQuery = useTenantUserRoles(tenantId, usersQuery.data);
	const userRoleMap = userRoleMapQuery.data ?? {};

	const catalogItemsByCode = useMemo(() => {
		const map = new Map<string, PermissionCatalogItemDTO>();
		permissionCatalogQuery.data?.items.forEach((item) => {
			map.set(item.code, item);
		});
		return map;
	}, [permissionCatalogQuery.data]);

	const categoryLabelMap = useMemo(() => {
		const result: Record<string, string> = {};
		const raw = permissionCatalogQuery.data?.categoryLabels ?? {};
		Object.entries(raw).forEach(([key, translations]) => {
			result[key] = translations?.[preferredLocale] ?? translations?.['zh-CN'] ?? key;
		});
		return result;
	}, [permissionCatalogQuery.data?.categoryLabels, preferredLocale]);

	const resolvedCategoryLabels = useMemo(() => ({ general: '通用', ...categoryLabelMap }), [categoryLabelMap]);

	const enrichPermission = (perm: TenantPermissionDTO): EnrichedPermission => {
		const catalogItem = catalogItemsByCode.get(perm.code);
		const localeEntry = catalogItem?.locales?.[preferredLocale]
			?? catalogItem?.locales?.['zh-CN']
			?? catalogItem?.locales?.['en-US'];
		return {
			...perm,
			displayName: localeEntry?.name ?? perm.name,
			displayDescription: localeEntry?.description ?? perm.description,
			catalogCategory: catalogItem?.category ?? perm.category ?? 'general',
		};
	};

	const enrichedPermissions = useMemo(() => (permissionsQuery.data ?? []).map(enrichPermission), [permissionsQuery.data, catalogItemsByCode, preferredLocale]);

	const filteredPermissions = useMemo(() => {
		const keyword = permissionSearch.trim().toLowerCase();
		if (!keyword) {
			return enrichedPermissions;
		}
		return enrichedPermissions.filter((perm) => {
			return (
				perm.code.toLowerCase().includes(keyword) ||
				perm.displayName.toLowerCase().includes(keyword) ||
				(perm.displayDescription?.toLowerCase().includes(keyword) ?? false)
			);
		});
	}, [enrichedPermissions, permissionSearch]);

	const groupedPermissions = useMemo(() => {
		const groups: Record<string, EnrichedPermission[]> = {};
		filteredPermissions.forEach((perm) => {
			const key = perm.catalogCategory || 'general';
			if (!groups[key]) {
				groups[key] = [];
			}
			groups[key].push(perm);
		});
		return groups;
	}, [filteredPermissions]);

	const orderedCategories = useMemo(() => {
		const base = permissionCatalogQuery.data?.categoryOrder ?? [];
		const extras = Object.keys(groupedPermissions).filter((key) => !base.includes(key));
		return [...base, ...extras];
	}, [groupedPermissions, permissionCatalogQuery.data?.categoryOrder]);

	const createRole = useMutation({
		mutationFn: (payload: UpsertRolePayload) => TenantAPI.createRole(tenantId ?? '', payload),
		onSuccess: async () => {
			await queryClient.invalidateQueries({ queryKey: ['tenant-roles', tenantId] });
			message.success('角色创建成功');
		},
	});
	const updateRole = useMutation({
		mutationFn: (payload: UpsertRolePayload & { id: string }) => TenantAPI.updateRole(tenantId ?? '', payload),
		onSuccess: async () => {
			await queryClient.invalidateQueries({ queryKey: ['tenant-roles', tenantId] });
			message.success('角色更新成功');
		},
	});
	const deleteRole = useMutation({
		mutationFn: (roleId: string) => TenantAPI.deleteRole(tenantId ?? '', roleId),
		onSuccess: async () => {
			await queryClient.invalidateQueries({ queryKey: ['tenant-roles', tenantId] });
			message.success('角色已删除');
		},
	});
	const replaceUserRoles = useMutation({
		mutationFn: (params: { userId: string; roleIds: string[] }) =>
			TenantAPI.replaceUserRoles(tenantId ?? '', params.userId, params.roleIds),
		onSuccess: async () => {
			await queryClient.invalidateQueries({ queryKey: USER_ROLE_QUERY_KEY });
			message.success('角色分配已更新');
		},
	});

	const roleColumns: ColumnsType<TenantRoleDTO> = [
		{ title: '名称', dataIndex: 'name', key: 'name' },
		{ title: '标识', dataIndex: 'code', key: 'code' },
		{ title: '系统角色', dataIndex: 'isSystem', key: 'isSystem', render: (value: boolean) => (value ? <Tag color="purple">系统</Tag> : <Tag>自定义</Tag>) },
		{ title: '权限数量', dataIndex: 'permissionIds', key: 'permissionIds', render: (ids: string[]) => ids?.length ?? 0 },
		{
			title: '操作',
			key: 'actions',
			render: (_, record) => (
				<Space>
					<Button type="link" onClick={() => openRoleModal(record)} disabled={!hasPermission(PERMISSIONS.MANAGE_ROLES)}>
						编辑
					</Button>
					<Button type="link" danger onClick={() => confirmDeleteRole(record)} disabled={record.isSystem || !hasPermission(PERMISSIONS.MANAGE_ROLES)}>
						删除
					</Button>
				</Space>
			),
		},
	];

	const userColumns: ColumnsType<TenantUserDTO> = [
		{ title: '邮箱', dataIndex: 'email', key: 'email' },
		{ title: '用户名', dataIndex: 'username', key: 'username' },
		{ title: '状态', dataIndex: 'status', key: 'status', render: (status: string) => <Tag color={status === 'active' ? 'green' : 'orange'}>{status}</Tag> },
		{
			title: '已分配角色',
			key: 'roles',
			render: (_, record) => {
				const assigned = userRoleMap[record.id];
				if (userRoleMapQuery.isFetching) {
					return <Tag color="default">同步中...</Tag>;
				}
				if (!assigned || !assigned.length) {
					return <Tag>未分配</Tag>;
				}
				return (
					<Space size={[4, 4]} wrap>
						{assigned.map((roleId) => {
							const role = rolesQuery.data?.find((item) => item.id === roleId);
							return <Tag key={roleId}>{role?.name ?? roleId}</Tag>;
						})}
					</Space>
				);
			},
		},
		{
			title: '操作',
			key: 'assign',
			render: (_, record) => (
				<Button type="link" onClick={() => openAssignModal(record)} disabled={!hasPermission(PERMISSIONS.MANAGE_ROLES)}>
					分配角色
				</Button>
			),
		},
	];

	const openRoleModal = (role?: TenantRoleDTO) => {
		setEditingRole(role ?? null);
		setRoleModalOpen(true);
		roleForm.setFieldsValue({
			name: role?.name ?? '',
			description: role?.description ?? '',
			permissionIds: role?.permissionIds ?? [],
		});
	};

	const handleSaveRole = async () => {
		const values = await roleForm.validateFields();
		if (editingRole) {
			await updateRole.mutateAsync({ ...values, id: editingRole.id });
		} else {
			await createRole.mutateAsync(values);
		}
		setRoleModalOpen(false);
	};

	const confirmDeleteRole = (role: TenantRoleDTO) => {
		Modal.confirm({
			title: `确定删除角色「${role.name}」吗？`,
			okText: '删除',
			okType: 'danger',
			onOk: () => deleteRole.mutateAsync(role.id),
		});
	};

	const openAssignModal = (record: TenantUserDTO) => {
		setAssignTarget(record);
		setAssignRoles(userRoleMap[record.id] ?? []);
	};

	const handleSaveAssignment = async () => {
		if (!assignTarget) return;
		await replaceUserRoles.mutateAsync({ userId: assignTarget.id, roleIds: assignRoles });
		setAssignTarget(null);
		setAssignRoles([]);
	};

	if (!tenantId) {
		return <Alert message="当前用户未绑定租户，无法管理角色" type="error" showIcon />;
	}

	return (
		<Card>
			<Title level={3}>角色与权限管理</Title>
			<Paragraph type="secondary">创建自定义角色、配置权限，并将其分配给租户成员。</Paragraph>
			<Alert
				style={{ marginBottom: 16 }}
				type="info"
				showIcon
				message={`当前角色：${currentRoles.length ? currentRoles.join(', ') : '未分配'}`}
				description={`可用权限点：${currentPermissions.length} 项`}
			/>
			<Tabs
				items={[
					{
						key: 'roles',
						label: '角色列表',
						children: (
							<>
								<Space style={{ marginBottom: 16 }}>
									<Button type="primary" onClick={() => openRoleModal()} disabled={!hasPermission(PERMISSIONS.MANAGE_ROLES)}>
										新建角色
									</Button>
								</Space>
								<Table
									rowKey="id"
									loading={rolesQuery.isLoading}
									columns={roleColumns}
									dataSource={rolesQuery.data}
									pagination={false}
								/>
							</>
						),
					},
					{
						key: 'users',
						label: '成员角色',
						children: (
							<>
								<Space style={{ marginBottom: 16 }}>
									<Button icon={<ReloadOutlined />} onClick={() => userRoleMapQuery.refetch()} loading={userRoleMapQuery.isFetching}>
										同步成员角色
									</Button>
								</Space>
								<Table
									rowKey="id"
									loading={usersQuery.isLoading}
									columns={userColumns}
									dataSource={usersQuery.data}
									pagination={{ pageSize: 10 }}
								/>
							</>
						),
					},
				]}
			/>
			<Card style={{ marginTop: 24 }} title="最近操作日志">
				<List
					loading={auditLogsQuery.isLoading}
					dataSource={auditLogs}
					locale={{ emptyText: '暂无操作记录' }}
					renderItem={(item: AuditLogDTO) => (
						<List.Item>
							<Space direction="vertical" style={{ width: '100%' }}>
								<Typography.Text strong>
									{item.action} · {item.resource}
								</Typography.Text>
								<Typography.Text type="secondary">
									{new Date(item.createdAt).toLocaleString()} · {item.userId ? `操作者 ${item.userId}` : '系统'}
								</Typography.Text>
								{item.details && (
									<Typography.Paragraph type="secondary">
										{JSON.stringify(item.details)}
									</Typography.Paragraph>
								)}
							</Space>
						</List.Item>
					)}
				/>
			</Card>
			<Modal
				title={editingRole ? '编辑角色' : '新建角色'}
				open={roleModalOpen}
				onCancel={() => setRoleModalOpen(false)}
				onOk={handleSaveRole}
				confirmLoading={createRole.isPending || updateRole.isPending}
			>
			<Form layout="vertical" form={roleForm} initialValues={{ permissionIds: [] }}>
					<Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入名称' }]}> 
						<Input placeholder="例如：审核员" />
					</Form.Item>
					<Form.Item name="description" label="描述">
						<Input.TextArea rows={2} placeholder="角色的职责说明" />
					</Form.Item>
				<Form.Item label="权限搜索">
					<Space direction="vertical" style={{ width: '100%' }}>
						<Input.Search
							allowClear
							placeholder="按名称、代码搜索权限"
							value={permissionSearch}
							onChange={(event) => setPermissionSearch(event.target.value)}
						/>
						{permissionCatalogQuery.isLoading && <Alert type="info" message="权限字典加载中..." showIcon />}
						{permissionCatalogQuery.isError && <Alert type="warning" message="权限字典加载失败，已回退到基础信息" showIcon />}
					</Space>
				</Form.Item>
				<Form.Item name="permissionIds" label="权限点" rules={[{ required: true, message: '请选择至少一个权限' }]}> 
					<PermissionChecklist
						grouped={groupedPermissions}
						orderedKeys={orderedCategories}
						categoryLabels={resolvedCategoryLabels}
					/>
					</Form.Item>
				</Form>
			</Modal>
			<Modal
				title={assignTarget ? `为 ${assignTarget.username} 分配角色` : '分配角色'}
				open={Boolean(assignTarget)}
				onCancel={() => {
					setAssignTarget(null);
					setAssignRoles([]);
				}}
				onOk={handleSaveAssignment}
				confirmLoading={replaceUserRoles.isPending}
			>
				<Select
					mode="multiple"
					style={{ width: '100%' }}
					placeholder="选择角色"
					value={assignRoles}
					onChange={setAssignRoles as (values: string[]) => void}
				>
					{(rolesQuery.data ?? []).map((role) => (
						<Select.Option key={role.id} value={role.id}>
							{role.name}
						</Select.Option>
					))}
				</Select>
			</Modal>
		</Card>
	);
};

export default RoleManagementPage;
