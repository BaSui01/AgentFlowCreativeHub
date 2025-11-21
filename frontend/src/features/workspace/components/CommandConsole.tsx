import { SendOutlined } from '@ant-design/icons';
import { Button, Card, Input, Select, Space, TreeSelect, Typography, message } from 'antd';
import { useQuery } from '@tanstack/react-query';
import React, { useMemo, useState } from 'react';
import { AXIOS_INSTANCE } from '@/api/instance';
import type { WorkspaceNodeDTO } from '../types';
import { toTreeData } from '../utils';
import type { AttachContextPayload } from '../types';

interface CommandConsoleProps {
  nodes?: WorkspaceNodeDTO[];
  onAttach: (payload: AttachContextPayload) => Promise<string | void>;
  sessionId?: string;
  onSessionChange?: (sessionId: string) => void;
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

export const CommandConsole: React.FC<CommandConsoleProps> = ({ nodes = [], onAttach, sessionId, onSessionChange }) => {
  const [selectedNodes, setSelectedNodes] = useState<string[]>([]);
  const [commands, setCommands] = useState<string[]>([]);
  const [notes, setNotes] = useState('');
  const [agentId, setAgentId] = useState<string>();
  const [loading, setLoading] = useState(false);

  const { data: agents } = useQuery<AgentListItem[]>({
    queryKey: ['agents-list'],
    queryFn: async () => {
      const resp = await AXIOS_INSTANCE.get('/api/agents');
      return resp.data?.items ?? [];
    },
  });

  const treeData = useMemo(() => toTreeData(nodes), [nodes]);

  const handleAttach = async () => {
    if (!agentId) {
      message.warning('请选择需要通知的智能体');
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
      if (session && onSessionChange) {
        onSessionChange(session);
      }
      message.success('上下文已注入');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '注入失败';
      message.error(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card size="small" style={{ marginTop: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Space style={{ width: '100%' }}>
          <Select
            style={{ flex: 1 }}
            placeholder="@ 选择智能体"
            options={(agents || []).map((agent) => ({ label: `@${agent.name}`, value: agent.id }))}
            value={agentId}
            onChange={setAgentId}
            showSearch
          />
          <Select
            mode="multiple"
            style={{ flex: 1 }}
            placeholder="/ 命令"
            options={slashCommands}
            value={commands}
            onChange={setCommands}
          />
        </Space>
        <TreeSelect
          treeCheckable
          showCheckedStrategy={TreeSelect.SHOW_PARENT}
          placeholder="选择要附加的文件"
          style={{ width: '100%' }}
          value={selectedNodes}
          treeData={treeData}
          onChange={(values) => setSelectedNodes(values as string[])}
        />
        <Input.TextArea rows={3} placeholder="补充说明 (/ 可选)" value={notes} onChange={(e) => setNotes(e.target.value)} />
        <Space align="center" style={{ width: '100%', justifyContent: 'space-between' }}>
          <Typography.Text type="secondary">
            会话ID：{sessionId ?? '发送后自动生成'}
          </Typography.Text>
          <Button type="primary" icon={<SendOutlined />} onClick={handleAttach} loading={loading}>
            注入上下文
          </Button>
        </Space>
      </Space>
    </Card>
  );
};
