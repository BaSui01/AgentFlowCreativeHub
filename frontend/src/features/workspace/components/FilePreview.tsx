import { SaveOutlined } from '@ant-design/icons';
import { Alert, Button, Input, Space, Typography } from 'antd';
import React, { useEffect, useRef } from 'react';
import type { WorkspaceFileDetailDTO } from '../types';

interface FilePreviewProps {
  data?: WorkspaceFileDetailDTO;
  saving?: boolean;
  onSave: (payload: { nodeId: string; content: string; versionId?: string }) => void;
  canEdit?: boolean;
}

export const FilePreview: React.FC<FilePreviewProps> = ({ data, saving, onSave, canEdit = true }) => {
  const contentRef = useRef(data?.version?.content ?? '');

  useEffect(() => {
    contentRef.current = data?.version?.content ?? '';
  }, [data?.version?.content]);

  if (!data?.node) {
    return <Alert message="请选择文件" type="info" showIcon />;
  }

  if (data.node.type !== 'file') {
    return <Alert message="请选择具体文件进行编辑" type="warning" showIcon />;
  }

  const handleSave = () => {
    if (!data.node.id) return;
    onSave({ nodeId: data.node.id, content: contentRef.current, versionId: data.version?.id });
  };

  const handleContentChange = (value: string) => {
    if (!canEdit) return;
    contentRef.current = value;
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <Space style={{ marginBottom: 12, justifyContent: 'space-between', width: '100%' }}>
        <div>
          <Typography.Text strong>{data.node.name}</Typography.Text>
          <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
            {data.node.nodePath}
          </Typography.Text>
        </div>
        <Button icon={<SaveOutlined />} type="primary" onClick={handleSave} loading={saving} disabled={!canEdit}>
          保存
        </Button>
      </Space>
      {!canEdit && (
        <Alert message="您没有编辑权限，内容以只读方式展示" type="info" showIcon style={{ marginBottom: 12 }} />
      )}
      <Input.TextArea
        key={data.version?.id ?? data.node.id}
        defaultValue={data.version?.content ?? ''}
        onChange={(e) => handleContentChange(e.target.value)}
        rows={20}
        style={{ flex: 1 }}
        placeholder="在此编辑内容"
        readOnly={!canEdit}
      />
    </div>
  );
};
