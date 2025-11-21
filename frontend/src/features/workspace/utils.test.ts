import { describe, expect, it } from 'vitest';
import { flattenNodeOptions, toTreeData } from './utils';
import type { WorkspaceNodeDTO } from './types';

const mockTree: WorkspaceNodeDTO[] = [
  { id: 'root', name: '根目录', type: 'folder', nodePath: 'root', children: [
    { id: 'child', name: '文件', type: 'file', nodePath: 'root/file' },
  ] },
];

describe('workspace utils', () => {
  it('converts nodes to tree data', () => {
    const tree = toTreeData(mockTree);
    expect(tree).toHaveLength(1);
    expect(tree[0].children?.length).toBe(1);
    expect(tree[0].children?.[0].isLeaf).toBe(true);
  });

  it('flattens node options', () => {
    const options = flattenNodeOptions(mockTree);
    expect(options.map((o) => o.value)).toEqual(['root', 'child']);
  });
});
