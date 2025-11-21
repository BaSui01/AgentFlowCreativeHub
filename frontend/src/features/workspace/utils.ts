import type { DataNode } from 'antd/es/tree';
import type { WorkspaceNodeDTO } from './types';

export function toTreeData(nodes: WorkspaceNodeDTO[]): DataNode[] {
  return nodes.map((node) => ({
    key: node.id,
    title: node.name,
    value: node.id,
    children: node.children ? toTreeData(node.children) : undefined,
    isLeaf: node.type === 'file',
  }));
}

export function flattenNodeOptions(nodes: WorkspaceNodeDTO[]): { label: string; value: string }[] {
  const result: { label: string; value: string }[] = [];
  const walk = (items: WorkspaceNodeDTO[]) => {
    items.forEach((item) => {
      result.push({ label: item.name, value: item.id });
      if (item.children) {
        walk(item.children);
      }
    });
  };
  walk(nodes);
  return result;
}
