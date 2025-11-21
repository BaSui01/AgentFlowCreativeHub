import { CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { Button, Card, Modal, Space, Tag, Typography } from 'antd';
import React from 'react';
import type { WorkspaceStagingFileDTO } from '../types';

interface StagingPanelProps {
  items?: WorkspaceStagingFileDTO[];
  onApprove: (id: string) => void;
  onReject: (id: string, reason: string) => void;
  approvingId?: string;
  rejectingId?: string;
}

export const StagingPanel: React.FC<StagingPanelProps> = ({ items = [], onApprove, onReject, approvingId, rejectingId }) => {
  const handleReject = (item: WorkspaceStagingFileDTO) => {
    let value = '';
    Modal.confirm({
      title: '填写驳回原因',
      content: (
        <textarea
          style={{ width: '100%', minHeight: 80 }}
          onChange={(e) => {
            value = e.target.value;
          }}
        />
      ),
      okText: '提交',
      onOk: () => {
        if (!value.trim()) {
          value = '未提供原因';
        }
        onReject(item.id, value);
      },
    });
  };

  return (
    <div style={{ maxHeight: 'calc(100vh - 220px)', overflow: 'auto' }}>
      {items.map((item) => (
        <Card size="small" key={item.id} style={{ marginBottom: 12 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Space align="center" style={{ justifyContent: 'space-between', width: '100%' }}>
              <Typography.Text strong>{item.suggestedName}</Typography.Text>
              <Tag color="blue">{item.fileType}</Tag>
            </Space>
            <Typography.Paragraph ellipsis={{ rows: 3 }}>
              {item.summary || '暂无摘要'}
            </Typography.Paragraph>
            <Space>
              <Button
                icon={<CheckOutlined />}
                type="primary"
                size="small"
                loading={approvingId === item.id}
                onClick={() => onApprove(item.id)}
              >
                通过
              </Button>
              <Button
                icon={<CloseOutlined />}
                size="small"
                loading={rejectingId === item.id}
                onClick={() => handleReject(item)}
              >
                驳回
              </Button>
            </Space>
          </Space>
        </Card>
      ))}
      {!items.length && <Typography.Text type="secondary">暂无待审核文件</Typography.Text>}
    </div>
  );
};
