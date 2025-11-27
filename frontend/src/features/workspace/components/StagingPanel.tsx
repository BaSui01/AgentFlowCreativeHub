import { CheckOutlined, CloseOutlined, EditOutlined } from '@ant-design/icons';
import { Button, Card, Modal, Space, Tag, Typography, Tooltip } from 'antd';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import 'dayjs/locale/zh-cn';
import React from 'react';
import type { WorkspaceStagingFileDTO } from '../types';

dayjs.extend(relativeTime);
dayjs.locale('zh-cn');

interface StagingPanelProps {
  items?: WorkspaceStagingFileDTO[];
  onApprove: (args: { id: string; reviewToken: string }) => void;
  onReject: (args: { id: string; reviewToken: string; reason: string }) => void;
  onRequestChanges: (args: { id: string; reviewToken: string; reason: string }) => void;
  approvingId?: string;
  rejectingId?: string;
  requestingId?: string;
  canReview?: boolean;
}

const statusColor: Record<string, string> = {
  awaiting_review: 'gold',
  awaiting_secondary_review: 'purple',
  archived: 'green',
  rejected: 'red',
  changes_requested: 'orange',
  failed: 'red',
};

const statusLabel: Record<string, string> = {
  awaiting_review: '待审核',
  awaiting_secondary_review: '二次审核',
  archived: '已归档',
  rejected: '已驳回',
  changes_requested: '待补充',
  failed: '归档失败',
};

export const StagingPanel: React.FC<StagingPanelProps> = ({ items = [], onApprove, onReject, onRequestChanges, approvingId, rejectingId, requestingId, canReview = true }) => {
  const resolveToken = (item: WorkspaceStagingFileDTO) => {
    if (item.status === 'awaiting_secondary_review') {
      return item.secondaryReviewToken || '';
    }
    return item.reviewToken || '';
  };

  const handleWithReason = (title: string, callback: (reason: string) => void) => {
    let pendingReason = '';
    Modal.confirm({
      title,
      content: (
        <textarea
          style={{ width: '100%', minHeight: 80 }}
          onChange={(e) => {
            pendingReason = e.target.value;
          }}
          disabled={!canReview}
        />
      ),
      okText: '提交',
      onOk: () => {
        const finalReason = pendingReason.trim() ? pendingReason : '未提供原因';
        callback(finalReason);
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
              <Space>
                <Tag color="blue">{item.fileType}</Tag>
                <Tag color={statusColor[item.status] || 'default'}>{statusLabel[item.status] || item.status}</Tag>
              </Space>
            </Space>
            <Typography.Paragraph ellipsis={{ rows: 3 }}>
              {item.summary || '暂无摘要'}
            </Typography.Paragraph>
            {item.slaExpiresAt && (
              <Typography.Text type="secondary">
                SLA 截止：{dayjs(item.slaExpiresAt).format('MM-DD HH:mm')}（剩余 {dayjs(item.slaExpiresAt).fromNow()}）
              </Typography.Text>
            )}
            <Typography.Text type="secondary">
              路径：{item.suggestedPath}
            </Typography.Text>
            {item.requiresSecondary && item.status === 'awaiting_review' && (
              <Typography.Text type="warning">该稿件需要二次审核，通过后将自动通知下一位审核人。</Typography.Text>
            )}
            <Typography.Text type="secondary">
              审核令牌：
              <Tooltip title="用于校验审核操作，复制后随请求一起提交">
                <span style={{ fontFamily: 'monospace' }}>{resolveToken(item) || '等待刷新'}</span>
              </Tooltip>
            </Typography.Text>
      <Space>
              <Button
                icon={<CheckOutlined />}
                type="primary"
                size="small"
                loading={approvingId === item.id}
          disabled={!resolveToken(item) || !canReview}
                onClick={() => onApprove({ id: item.id, reviewToken: resolveToken(item) })}
              >
                通过
              </Button>
              <Button
                icon={<EditOutlined />}
                size="small"
                loading={requestingId === item.id}
          disabled={!resolveToken(item) || !canReview}
                onClick={() =>
                  handleWithReason('填写待补充说明', (reason) =>
                    onRequestChanges({ id: item.id, reviewToken: resolveToken(item), reason }),
                  )
                }
              >
                请求补充
              </Button>
              <Button
                icon={<CloseOutlined />}
                size="small"
                loading={rejectingId === item.id}
          disabled={!resolveToken(item) || !canReview}
                onClick={() =>
                  handleWithReason('填写驳回原因', (reason) =>
                    onReject({ id: item.id, reviewToken: resolveToken(item), reason }),
                  )
                }
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
