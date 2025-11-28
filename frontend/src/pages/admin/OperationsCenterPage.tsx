import React, { useMemo, useState } from 'react';
import { Dayjs } from 'dayjs';
import { Alert, Button, Card, Col, DatePicker, Descriptions, Row, Select, Space, Statistic, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useQuery } from '@tanstack/react-query';

import { AXIOS_INSTANCE } from '@/api/instance';
import { useAuth } from '@/features/auth/model/auth-context';
import { useAuthorization, PERMISSIONS } from '@/features/auth/model/use-authorization';
import { CommandConsole, FileExplorer, StagingPanel } from '@/features/workspace/components';
import { useWorkspaceMutations, useWorkspaceStaging, useWorkspaceTree } from '@/features/workspace/hooks';
import { WorkspaceAPI } from '@/features/workspace/api';
import type { AttachContextPayload, CommandRequestDTO, CommandStatus } from '@/features/workspace/types';
const { RangePicker } = DatePicker;

const { Title, Paragraph } = Typography;

interface AgentListItem {
	id: string;
	name: string;
}

const COMMAND_PAGE_SIZE = 10;

const commandStatusColorMap: Record<CommandStatus, string> = {
	queued: 'gold',
	running: 'blue',
	completed: 'green',
	failed: 'red',
};

const commandStatusLabelMap: Record<CommandStatus, string> = {
	queued: '排队中',
	running: '执行中',
	completed: '已完成',
	failed: '执行失败',
};

const stagingStatusColorMap: Record<string, string> = {
	approved_pending_archive: 'green',
	archived: 'default',
	changes_requested: 'orange',
	rejected: 'red',
	failed: 'red',
};

const stagingStatusLabelMap: Record<string, string> = {
	approved_pending_archive: '待归档',
	archived: '已归档',
	changes_requested: '需修改',
	rejected: '已驳回',
	failed: '失败',
	awaiting_review: '待初审',
	awaiting_secondary_review: '待复审',
	drafted: '草稿',
};

const commandStatusOptions = [
	{ label: '全部状态', value: '' },
	{ label: commandStatusLabelMap.queued, value: 'queued' },
	{ label: commandStatusLabelMap.running, value: 'running' },
	{ label: commandStatusLabelMap.completed, value: 'completed' },
	{ label: commandStatusLabelMap.failed, value: 'failed' },
];

const parseTimestamp = (value?: string) => (value ? Date.parse(value) || 0 : 0);

export const OperationsCenterPage: React.FC = () => {
	const { user } = useAuth();
	const { hasPermission, roles, permissions } = useAuthorization();
	const canReview = hasPermission(PERMISSIONS.WORKSPACE_REVIEW);
	const canExecute = hasPermission(PERMISSIONS.COMMAND_EXECUTE);
	const treeQuery = useWorkspaceTree();
	const stagingQuery = useWorkspaceStaging();
	const mutations = useWorkspaceMutations();
	const [sessionId, setSessionId] = useState<string>();
	const [approvingId, setApprovingId] = useState<string>();
	const [rejectingId, setRejectingId] = useState<string>();
	const [requestingId, setRequestingId] = useState<string>();
	const [selectedNodeId, setSelectedNodeId] = useState<string>();
	const [latestCommand, setLatestCommand] = useState<CommandRequestDTO>();
	const [statusFilter, setStatusFilter] = useState<string>();
	const [agentFilter, setAgentFilter] = useState<string>();
	const [page, setPage] = useState(1);
	const [reviewRange, setReviewRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);

	const { data: agents } = useQuery<AgentListItem[]>({
		queryKey: ['agents-list'],
		queryFn: async () => {
			const resp = await AXIOS_INSTANCE.get('/api/agents');
			return resp.data?.items ?? [];
		},
	});

	const commandListQuery = useQuery({
		queryKey: ['commands-list', statusFilter, agentFilter, page],
		queryFn: () => WorkspaceAPI.listCommands({ status: statusFilter, agentId: agentFilter, page, pageSize: COMMAND_PAGE_SIZE }),
	});

	const handleApprove = (args: { id: string; reviewToken: string }) => {
		setApprovingId(args.id);
		mutations.approveStaging.mutate(args, {
			onSuccess: () => message.success('审核通过'),
			onSettled: () => setApprovingId(undefined),
		});
	};

	const handleReject = (args: { id: string; reviewToken: string; reason: string }) => {
		setRejectingId(args.id);
		mutations.rejectStaging.mutate(args, {
			onSuccess: () => message.success('已驳回'),
			onSettled: () => setRejectingId(undefined),
		});
	};

	const handleRequestChanges = (args: { id: string; reviewToken: string; reason: string }) => {
		setRequestingId(args.id);
		mutations.requestChanges.mutate(args, {
			onSuccess: () => message.success('已发出修改请求'),
			onSettled: () => setRequestingId(undefined),
		});
	};

	const handleAttachContext = async (payload: AttachContextPayload) => {
		const session = await WorkspaceAPI.attachContext(payload);
		if (session) {
			setSessionId(session);
			return session;
		}
		return sessionId ?? '';
	};

	const actionableStaging = (stagingQuery.data || []).filter((item) =>
		item.status === 'awaiting_review' || item.status === 'awaiting_secondary_review',
	);

	const treeStats = useMemo(() => {
		const nodes = treeQuery.data ?? [];
		let fileCount = 0;
		nodes.forEach((node) => {
			if (node.type === 'file') {
				fileCount += 1;
			}
		});
		return { rootCount: nodes.length, fileCount };
	}, [treeQuery.data]);

	const commandData = commandListQuery.data ?? { items: [] as CommandRequestDTO[], total: 0, page: page, pageSize: COMMAND_PAGE_SIZE };

	const handleExportCommands = () => {
		if (!commandData.items.length) {
			message.info('暂无命令数据可导出');
			return;
		}
		const headers = ['ID', '智能体', '类型', '状态', '队列位置', '耗时(ms)', 'Token 消耗', '创建时间'];
		const escapeCsv = (value: unknown) => {
			const text = value === undefined || value === null ? '' : String(value);
			if (text.includes(',') || text.includes('"') || text.includes('\n')) {
				return `"${text.replace(/"/g, '""')}"`;
			}
			return text;
		};
		const rows = commandData.items.map((item) =>
			[
				item.id,
				item.agentId,
				item.commandType ?? '-',
				commandStatusLabelMap[item.status] ?? item.status,
				item.queuePosition ?? '-',
				item.latencyMs ?? '-',
				item.tokenCost ?? '-',
				item.createdAt ? new Date(item.createdAt).toISOString() : '-',
			]
				.map(escapeCsv)
				.join(','),
		);
		const csvContent = [headers.join(','), ...rows].join('\n');
		const blob = new Blob([`\uFEFF${csvContent}`], { type: 'text/csv;charset=utf-8;' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = `commands-${Date.now()}.csv`;
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		URL.revokeObjectURL(url);
	};

	const commandColumns: ColumnsType<CommandRequestDTO> = [
		{ title: '命令ID', dataIndex: 'id', key: 'id', width: 200 },
		{ title: '类型', dataIndex: 'commandType', key: 'commandType', ellipsis: true },
		{ title: '智能体', dataIndex: 'agentId', key: 'agentId', render: (value: string) => value || '-' },
		{
			title: '状态',
			dataIndex: 'status',
			key: 'status',
			render: (status: CommandStatus) => (
				<Tag color={commandStatusColorMap[status] ?? 'default'}>{commandStatusLabelMap[status] ?? status}</Tag>
			),
		},
		{ title: '队列', dataIndex: 'queuePosition', key: 'queuePosition', render: (value?: number) => (value ? `#${value}` : '-') },
		{ title: '耗时(ms)', dataIndex: 'latencyMs', key: 'latencyMs', render: (value?: number) => (value ? value : '-') },
		{ title: 'Token', dataIndex: 'tokenCost', key: 'tokenCost', render: (value?: number) => (value ? value : '-') },
		{ title: '输出预览', dataIndex: 'resultPreview', key: 'resultPreview', ellipsis: true },
		{ title: '失败原因', dataIndex: 'failureReason', key: 'failureReason', ellipsis: true },
		{
			title: '创建时间',
			dataIndex: 'createdAt',
			key: 'createdAt',
			render: (value?: string) => (value ? new Date(value).toLocaleString() : '-'),
		},
	];

	const reviewHistory = useMemo(() => {
		const items = stagingQuery.data ?? [];
		const [from, to] = reviewRange;
		return items
			.filter((item) => item.status && !['awaiting_review', 'awaiting_secondary_review'].includes(item.status))
			.filter((item) => {
				const ts = parseTimestamp(item.createdAt);
				if (from && ts < from.startOf('day').valueOf()) {
					return false;
				}
				if (to && ts > to.endOf('day').valueOf()) {
					return false;
				}
				return true;
			})
			.sort((a, b) => parseTimestamp(b.createdAt) - parseTimestamp(a.createdAt))
			.slice(0, 5);
	}, [stagingQuery.data, reviewRange]);

	const lastLoginFallback = (user as unknown as { last_login_at?: string; lastLoginAt?: string } | undefined)?.last_login_at
		?? (user as unknown as { last_login_at?: string; lastLoginAt?: string } | undefined)?.lastLoginAt
		?? '-';

	return (
		<Card>
			<Title level={3}>运营控制台</Title>
			<Paragraph type="secondary">在此执行高权限操作，例如审核稿件与调度智能体命令。</Paragraph>
			<Descriptions size="small" column={1} style={{ marginBottom: 16 }}>
				<Descriptions.Item label="当前账号">{user?.email ?? user?.username ?? '-'}</Descriptions.Item>
				<Descriptions.Item label="所属租户">{(user as { tenant_name?: string; tenant_id?: string } | undefined)?.tenant_name ?? (user?.tenant_id ?? '-')}</Descriptions.Item>
				<Descriptions.Item label="角色">{roles.length ? roles.join(', ') : '未分配'}</Descriptions.Item>
				<Descriptions.Item label="权限数量">{permissions.length}</Descriptions.Item>
				<Descriptions.Item label="上次登录">{lastLoginFallback}</Descriptions.Item>
			</Descriptions>
			<Row gutter={16} style={{ marginBottom: 24 }}>
				<Col xs={24} md={8}>
					<Card size="small">
						<Statistic title="待审核稿件" value={actionableStaging.length} suffix="条" loading={stagingQuery.isFetching} />
					</Card>
				</Col>
				<Col xs={24} md={8}>
					<Card size="small">
						<Statistic title="工作区节点" value={treeStats.rootCount} loading={treeQuery.isFetching} />
					</Card>
				</Col>
				<Col xs={24} md={8}>
					<Card size="small">
						<Statistic title="文件数量" value={treeStats.fileCount} loading={treeQuery.isFetching} />
					</Card>
				</Col>
			</Row>
			<Row gutter={16}>
				<Col xs={24} md={8}>
					<FileExplorer
						nodes={treeQuery.data}
						loading={treeQuery.isFetching}
						onSelect={setSelectedNodeId}
						onCreateFolder={() => message.warning('请在工作区中创建目录')} // 管理功能仅提供浏览
						onRefresh={() => treeQuery.refetch()}
						selectedNodeId={selectedNodeId}
						canManage={false}
					/>
				</Col>
				<Col xs={24} md={16}>
					{canReview ? (
						<StagingPanel
							items={actionableStaging}
							onApprove={handleApprove}
							onReject={handleReject}
							onRequestChanges={handleRequestChanges}
							approvingId={approvingId}
							rejectingId={rejectingId}
							requestingId={requestingId}
							canReview
						/>
					) : (
						<Alert message="需要审核权限才能访问暂存区" type="warning" showIcon />
					)}
				</Col>
			</Row>
			<Card size="small" style={{ marginTop: 24 }} title="命令列表" extra={<Button onClick={handleExportCommands}>导出 CSV</Button>}>
				<Space wrap style={{ marginBottom: 16 }}>
					<Select
						style={{ width: 160 }}
						placeholder="按状态筛选"
						options={commandStatusOptions}
						value={statusFilter ?? ''}
						onChange={(value) => {
							setPage(1);
							setStatusFilter(value || undefined);
						}}
						allowClear
					/>
					<Select
						style={{ width: 200 }}
						placeholder="按智能体筛选"
						options={(agents ?? []).map((agent) => ({ label: `@${agent.name}`, value: agent.id }))}
						value={agentFilter}
						onChange={(value) => {
							setPage(1);
							setAgentFilter(value);
						}}
						allowClear
						showSearch
					/>
					<Button onClick={() => commandListQuery.refetch()} loading={commandListQuery.isFetching}>
						刷新
					</Button>
				</Space>
				<Table
					rowKey="id"
					columns={commandColumns}
					dataSource={commandData.items}
					loading={commandListQuery.isLoading}
					pagination={{
						current: commandData.page,
						pageSize: COMMAND_PAGE_SIZE,
						total: commandData.total,
						onChange: (nextPage) => setPage(nextPage),
					}}
					scroll={{ x: 800 }}
				/>
			</Card>
			<Card
				size="small"
				style={{ marginTop: 16 }}
				title="审核历史"
				extra={
					<Space>
						<RangePicker
							allowClear
							value={reviewRange as [Dayjs | null, Dayjs | null]}
							onChange={(values) => setReviewRange(values as [Dayjs | null, Dayjs | null])}
						/>
						<Button onClick={() => setReviewRange([null, null])}>重置</Button>
					</Space>
				}
			>
				{reviewHistory.length ? (
					<Space direction="vertical" style={{ width: '100%' }}>
						{reviewHistory.map((item) => (
							<Card key={item.id} size="small">
								<Space direction="vertical" style={{ width: '100%' }}>
									<Space align="center" style={{ justifyContent: 'space-between' }}>
										<span>{item.suggestedName}</span>
										<Tag color={stagingStatusColorMap[item.status] ?? 'default'}>
											{stagingStatusLabelMap[item.status] ?? item.status}
										</Tag>
									</Space>
									<Typography.Text type="secondary">
										{item.summary ?? '暂无摘要'}
									</Typography.Text>
									<Typography.Text type="secondary">
										审核人：{item.reviewer_id ?? item.secondaryReviewerId ?? '未记录'}
									</Typography.Text>
									<Typography.Text type="secondary">
										{item.createdAt ? new Date(item.createdAt).toLocaleString() : '-'}
									</Typography.Text>
								</Space>
							</Card>
						))}
					</Space>
				) : (
					<Alert type="info" message="暂无审核历史" showIcon />
				)}
			</Card>
			<div style={{ marginTop: 24 }}>
				{canExecute ? (
					<CommandConsole
						nodes={treeQuery.data}
						onAttach={handleAttachContext}
						sessionId={sessionId}
						onSessionChange={setSessionId}
						canExecute
						onCommandCreated={setLatestCommand}
					/>
				) : (
					<Alert message="需要命令执行权限才能调度智能体" type="warning" showIcon />
				)}
				{latestCommand && (
					<Card size="small" style={{ marginTop: 16 }} title="最近提交的命令">
						<Descriptions size="small" column={1}>
							<Descriptions.Item label="ID">{latestCommand.id}</Descriptions.Item>
							<Descriptions.Item label="状态">
								<Tag color={{ queued: 'gold', running: 'blue', completed: 'green', failed: 'red' }[latestCommand.status] ?? 'default'}>
									{latestCommand.status}
								</Tag>
							</Descriptions.Item>
							{latestCommand.resultPreview && (
								<Descriptions.Item label="结果预览">{latestCommand.resultPreview}</Descriptions.Item>
							)}
							{latestCommand.failureReason && (
								<Descriptions.Item label="失败原因">{latestCommand.failureReason}</Descriptions.Item>
							)}
						</Descriptions>
					</Card>
				)}
			</div>
		</Card>
	);
};

export default OperationsCenterPage;
