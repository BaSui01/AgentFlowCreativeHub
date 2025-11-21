import React, { useState } from 'react';
import { Card, Col, Row, Statistic, Tag, Typography, Modal, Input, message } from 'antd';
import { useQuery } from '@tanstack/react-query';

import { SystemService } from '@/shared/api';
import { useAuth } from '@/features/auth/model/auth-context';
import { FileExplorer, FilePreview, StagingPanel, CommandConsole } from '@/features/workspace/components';
import { WorkspaceAPI } from '@/features/workspace/api';
import { useWorkspaceTree, useWorkspaceFile, useWorkspaceStaging, useWorkspaceMutations } from '@/features/workspace/hooks';
import type { AttachContextPayload } from '@/features/workspace/types';

const { Title, Paragraph, Text } = Typography;

export const DashboardPage: React.FC = () => {
  const { user } = useAuth();
  const { data: health, isLoading: loadingHealth } = useQuery({
    queryKey: ['system-health'],
    queryFn: () => SystemService.getHealth(),
    staleTime: 30_000,
  });

  const { data: readiness, isLoading: loadingReady } = useQuery({
    queryKey: ['system-ready'],
    queryFn: () => SystemService.getReady(),
    staleTime: 30_000,
  });

  const [selectedNodeId, setSelectedNodeId] = useState<string>();
  const [sessionId, setSessionId] = useState<string>();
  const [approvingId, setApprovingId] = useState<string>();
  const [rejectingId, setRejectingId] = useState<string>();

  const treeQuery = useWorkspaceTree();
  const fileQuery = useWorkspaceFile(selectedNodeId);
  const stagingQuery = useWorkspaceStaging();
  const mutations = useWorkspaceMutations();

  const handleCreateFolder = () => {
    let value = '';
    Modal.confirm({
      title: '新建文件夹',
      content: <Input autoFocus placeholder="请输入文件夹名称" onChange={(e) => (value = e.target.value)} />,
      okText: '创建',
      onOk: async () => {
        const name = value.trim();
        if (!name) {
          message.warning('名称不能为空');
          return Promise.reject();
        }
        await mutations.createFolder.mutateAsync({ name });
        message.success('已创建');
      },
    });
  };

  const handleSaveFile = (payload: { nodeId: string; content: string }) => {
    mutations.saveFile.mutate(payload, {
      onSuccess: () => message.success('已保存'),
    });
  };

  const handleApprove = (id: string) => {
    setApprovingId(id);
    mutations.approveStaging.mutate(id, {
      onSuccess: () => message.success('审核通过'),
      onSettled: () => setApprovingId(undefined),
    });
  };

  const handleReject = (id: string, reason: string) => {
    setRejectingId(id);
    mutations.rejectStaging.mutate(
      { id, reason },
      {
        onSuccess: () => message.success('已驳回'),
        onSettled: () => setRejectingId(undefined),
      },
    );
  };

  const handleAttachContext = async (payload: AttachContextPayload) => {
    const session = await WorkspaceAPI.attachContext(payload);
    if (session) {
      setSessionId(session);
      return session;
    }
    return sessionId ?? '';
  };

  return (
    <div>
      <Title level={2}>欢迎，{user?.name ?? user?.email ?? '管理员'}</Title>
      <Paragraph type="secondary">这里是多租户代理执行与监控的总控面板。</Paragraph>

      <Row gutter={16}>
        <Col xs={24} md={12} lg={8}>
          <Card loading={loadingHealth} title="系统健康状态">
            <Tag color={health?.status === 'healthy' ? 'green' : 'red'}>{health?.status ?? '未知'}</Tag>
            <Paragraph style={{ marginTop: 16 }}>服务：{health?.service ?? 'AgentFlow'}</Paragraph>
          </Card>
        </Col>
        <Col xs={24} md={12} lg={8}>
          <Card loading={loadingReady} title="依赖自检">
            <Tag color={readiness?.status === 'ready' ? 'blue' : 'red'}>{readiness?.status ?? '检测中'}</Tag>
            <Paragraph style={{ marginTop: 16 }}>数据库：{readiness?.database ?? readiness?.reason ?? '检测中'}</Paragraph>
          </Card>
        </Col>
        <Col xs={24} md={12} lg={8}>
          <Card title="令牌信息">
            <Statistic title="角色" value={user?.roles?.join(', ') || '未分配'} />
            <Text type="secondary">Tenant ID：{user?.tenant_id ?? '未知'}</Text>
          </Card>
        </Col>
      </Row>

      <Card title="智能文件工作区" style={{ marginTop: 24 }} bodyStyle={{ paddingBottom: 16 }}>
        <Row gutter={16}>
          <Col xs={24} lg={6}>
            <FileExplorer
              nodes={treeQuery.data}
              loading={treeQuery.isFetching}
              selectedNodeId={selectedNodeId}
              onSelect={setSelectedNodeId}
              onCreateFolder={handleCreateFolder}
              onRefresh={() => treeQuery.refetch()}
            />
          </Col>
          <Col xs={24} lg={10} style={{ borderRight: '1px solid #f5f5f5', padding: '0 16px' }}>
            <FilePreview data={fileQuery.data} saving={mutations.saveFile.isPending} onSave={handleSaveFile} />
          </Col>
          <Col xs={24} lg={8}>
            <StagingPanel
              items={stagingQuery.data}
              onApprove={handleApprove}
              onReject={handleReject}
              approvingId={approvingId}
              rejectingId={rejectingId}
            />
          </Col>
        </Row>
        <CommandConsole
          nodes={treeQuery.data}
          onAttach={handleAttachContext}
          sessionId={sessionId}
          onSessionChange={setSessionId}
        />
      </Card>
    </div>
  );
};
