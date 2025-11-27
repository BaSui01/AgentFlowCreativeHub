import { SendOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Input, Select, Space, Spin, Tag, TreeSelect, Typography, message } from 'antd';
import { useQuery } from '@tanstack/react-query';
import React, { useMemo, useState } from 'react';
import { AXIOS_INSTANCE } from '@/api/instance';
import { WorkspaceAPI } from '../api';
import { useCommandStatus } from '../hooks';
import type { WorkspaceNodeDTO, CommandRequestDTO, CommandStatus } from '../types';
import { toTreeData } from '../utils';
import type { AttachContextPayload } from '../types';

interface CommandConsoleProps {
  nodes?: WorkspaceNodeDTO[];
  onAttach: (payload: AttachContextPayload) => Promise<string | void>;
  sessionId?: string;
  onSessionChange?: (sessionId: string) => void;
  canExecute?: boolean;
  onCommandCreated?: (command: CommandRequestDTO) => void;
}

interface AgentListItem {
  id: string;
  name: string;
}

const slashCommands = [
  { label: '/总结', value: '/总结' },
  { label: '/润色', value: '/润色' },
  { label: '/引用大纲', value: '/引用大纲' },
  { label: '/同步素材', value: '/同步素材' },
];

const statusColorMap: Record<CommandStatus, string> = {
  queued: 'gold',
  running: 'blue',
  completed: 'green',
  failed: 'red',
};

const statusLabelMap: Record<CommandStatus, string> = {
  queued: '排队中',
  running: '执行中',
  completed: '已完成',
  failed: '执行失败',
};

export const CommandConsole: React.FC<CommandConsoleProps> = ({ nodes = [], onAttach, sessionId, onSessionChange, canExecute = true, onCommandCreated }) => {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [commands, setCommands] = useState<string[]>([]);
  const [notes, setNotes] = useState('');
  const [commandContent, setCommandContent] = useState('');
  const [agentId, setAgentId] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [currentCommandId, setCurrentCommandId] = useState<string>();
  const [localCommand, setLocalCommand] = useState<CommandRequestDTO>();

  const { command: remoteCommand, isLoading: statusLoading } = useCommandStatus(currentCommandId);
  const activeCommand = remoteCommand ?? localCommand;

  const { data: agents } = useQuery<AgentListItem[]>({
    queryKey: ['agents-list'],
    queryFn: async () => {
      const resp = await AXIOS_INSTANCE.get('/api/agents');
      return resp.data?.items ?? [];
    },
  });

  const treeData = useMemo(() => toTreeData(nodes), [nodes]);

  const renderStatusTag = (status?: CommandStatus) => {
    if (!status) {
      return <Tag>未知</Tag>;
    }
    return <Tag color={statusColorMap[status]}>{statusLabelMap[status]}</Tag>;
  };

  const handleExecuteCommand = async () => {
    if (!canExecute) {
      message.warning('当前账号无权执行命令');
      return;
    }
    if (!agentId) {
      message.warning('请选择需要通知的智能体');
      return;
    }
    const trimmedCommand = commandContent.trim();
    if (!trimmedCommand) {
      message.warning('请输入要执行的命令');
      return;
    }
    setLoading(true);
    try {
      const selectedAgent = (agents || []).find((agent) => agent.id === agentId);
      const session = await onAttach({
        agentId,
        sessionId,
        nodeIds: selectedNodes,
        mentions: selectedAgent ? [`@${selectedAgent.name}`] : [],
        commands,
        notes,
      });
      const resolvedSession = session ?? sessionId;
      if (resolvedSession && onSessionChange) {
        onSessionChange(resolvedSession);
      }

      const contentPrefix = commands.join(' ').trim();
      const combinedContent = contentPrefix ? `${contentPrefix} ${trimmedCommand}`.trim() : trimmedCommand;
      const response = await WorkspaceAPI.executeCommand({
        agentId,
        commandType: commands[0] ?? 'custom',
        content: combinedContent,
        contextNodeIds: selectedNodes,
        sessionId: resolvedSession,
        notes,
      });

      if (response?.request) {
        setCurrentCommandId(response.request.id);
        setLocalCommand(response.request);
        setCommandContent('');
        onCommandCreated?.(response.request);
        message.success(response.new ? '命令已入队执行' : '已复用最近一次执行结果');
      } else {
        message.warning('命令已提交，但暂未返回状态');
      }
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '执行命令失败';
      message.error(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card size="small" style={{ marginTop: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
        {!canExecute && (
          <Alert type="info" message="您没有执行命令的权限" showIcon />
        )}
        <Space style={{ width: '100%' }}>
          <Select
            style={{ flex: 1 }}
            placeholder="@ 选择智能体"
            options={(agents || []).map((agent) => ({ label: `@${agent.name}`, value: agent.id }))}
            value={agentId}
            onChange={setAgentId}
            showSearch
            disabled={!canExecute}
          />
          <Select
            mode="multiple"
            style={{ flex: 1 }}
            placeholder="/ 命令"
            options={slashCommands}
            value={commands}
            onChange={setCommands}
            disabled={!canExecute}
          />
        </Space>
        <TreeSelect
          treeCheckable
          showCheckedStrategy={TreeSelect.SHOW_PARENT}
          placeholder="选择要附加的文件"
          style={{ width: '100%' }}
          value={selectedNodes}
          treeData={treeData}
          onChange={(values) => {
            if (!canExecute) return;
            setSelectedNodes(values as string[]);
          }}
          disabled={!canExecute}
        />
        <Input.TextArea rows={3} placeholder="请输入要执行的命令" value={commandContent} onChange={(e) => setCommandContent(e.target.value)} disabled={!canExecute} />
        <Input.TextArea rows={3} placeholder="补充说明 (/ 可选)" value={notes} onChange={(e) => setNotes(e.target.value)} disabled={!canExecute} />
        <Space align="center" style={{ width: '100%', justifyContent: 'space-between' }}>
          <Typography.Text type="secondary">
            会话ID：{sessionId ?? '发送后自动生成'}
          </Typography.Text>
          <Button type="primary" icon={<SendOutlined />} onClick={handleExecuteCommand} loading={loading} disabled={!canExecute}>
            注入并执行
          </Button>
        </Space>
        {activeCommand && (
          <Card size="small" type="inner" title="命令执行状态">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space align="center">
                <Typography.Text strong>当前状态：</Typography.Text>
                {renderStatusTag(activeCommand.status)}
                {statusLoading && <Spin size="small" />}
              </Space>
              {activeCommand.status === 'queued' && typeof activeCommand.queuePosition === 'number' && (
                <Typography.Text type="secondary">队列位置：第 {activeCommand.queuePosition} 位</Typography.Text>
              )}
              {activeCommand.traceId && (
                <Typography.Text type="secondary">Trace ID：{activeCommand.traceId}</Typography.Text>
              )}
              {activeCommand.contextSnapshot && (
                <Typography.Paragraph style={{ whiteSpace: 'pre-wrap' }}>
                  {activeCommand.contextSnapshot}
                </Typography.Paragraph>
              )}
              {activeCommand.notes && (
                <Typography.Text type={activeCommand.notes.includes('截断') ? 'warning' : undefined}>
                  备注：{activeCommand.notes}
                </Typography.Text>
              )}
              {activeCommand.status === 'completed' && activeCommand.resultPreview && (
                <Typography.Paragraph style={{ whiteSpace: 'pre-wrap' }}>
                  结果预览：{activeCommand.resultPreview}
                </Typography.Paragraph>
              )}
              {activeCommand.status === 'failed' && activeCommand.failureReason && (
                <Typography.Text type="danger" style={{ whiteSpace: 'pre-wrap' }}>
                  失败原因：{activeCommand.failureReason}
                </Typography.Text>
              )}
              <Space size="large">
                {typeof activeCommand.latencyMs === 'number' && (
                  <Typography.Text type="secondary">耗时：{activeCommand.latencyMs}ms</Typography.Text>
                )}
                {typeof activeCommand.tokenCost === 'number' && (
                  <Typography.Text type="secondary">Token：{activeCommand.tokenCost}</Typography.Text>
                )}
              </Space>
            </Space>
          </Card>
        )}
      </Space>
    </Card>
  );
};
