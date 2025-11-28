import { FolderAddOutlined, ReloadOutlined } from '@ant-design/icons';
import { Button, Empty, Space, Tree } from 'antd';
import type { TreeProps } from 'antd/es/tree';
import React from 'react';
import type { WorkspaceNodeDTO } from '../types';
import { toTreeData } from '../utils';

const { DirectoryTree } = Tree;

interface FileExplorerProps {
  nodes?: WorkspaceNodeDTO[];
  loading?: boolean;
  onSelect: (nodeId: string) => void;
  selectedNodeId?: string;
  onCreateFolder: () => void;
  onRefresh: () => void;
  canManage?: boolean;
}

export const FileExplorer: React.FC<FileExplorerProps> = ({
  nodes = [],
  loading,
  selectedNodeId,
  onSelect,
  onCreateFolder,
  onRefresh,
  canManage = true,
}) => {
  const treeData = toTreeData(nodes);

  const handleSelect: TreeProps['onSelect'] = (keys) => {
    const id = keys[0] as string;
    if (id) {
      onSelect(id);
    }
  };

  return (
    <div style={{ borderRight: '1px solid #f0f0f0', paddingRight: 12 }}>
      <Space style={{ marginBottom: 12 }}>
        <Button icon={<FolderAddOutlined />} onClick={onCreateFolder} size="small" disabled={!canManage}>
          新建文件夹
        </Button>
        <Button icon={<ReloadOutlined />} onClick={onRefresh} size="small" loading={loading}>
          刷新
        </Button>
      </Space>
      {treeData.length === 0 ? (
        <Empty description="暂无文件" />
      ) : (
        <DirectoryTree
          multiple={false}
          showIcon={false}
          defaultExpandAll
          selectedKeys={selectedNodeId ? [selectedNodeId] : []}
          onSelect={handleSelect}
          treeData={treeData}
          style={{ maxHeight: 'calc(100vh - 240px)', overflow: 'auto' }}
        />
      )}
    </div>
  );
};
