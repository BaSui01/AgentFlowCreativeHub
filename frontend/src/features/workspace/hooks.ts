import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { WorkspaceAPI } from './api';
import { WorkspaceFileDetailDTO, WorkspaceNodeDTO, WorkspaceStagingFileDTO } from './types';

export const workspaceKeys = {
  tree: ['workspace-tree'] as const,
  file: (id?: string) => ['workspace-file', id] as const,
  staging: ['workspace-staging'] as const,
};

export function useWorkspaceTree() {
  return useQuery<WorkspaceNodeDTO[]>({
    queryKey: workspaceKeys.tree,
    queryFn: () => WorkspaceAPI.fetchTree(),
  });
}

export function useWorkspaceFile(nodeId?: string) {
  return useQuery<WorkspaceFileDetailDTO | undefined>({
    queryKey: workspaceKeys.file(nodeId),
    enabled: Boolean(nodeId),
    queryFn: () => (nodeId ? WorkspaceAPI.getFile(nodeId) : Promise.resolve(undefined)),
  });
}

export function useWorkspaceStaging() {
  return useQuery<WorkspaceStagingFileDTO[]>({
    queryKey: workspaceKeys.staging,
    queryFn: () => WorkspaceAPI.listStaging('pending'),
  });
}

export function useWorkspaceMutations() {
  const queryClient = useQueryClient();

  const refreshTree = () => queryClient.invalidateQueries({ queryKey: workspaceKeys.tree });
  const refreshStaging = () => queryClient.invalidateQueries({ queryKey: workspaceKeys.staging });

  const createFolder = useMutation({
    mutationFn: WorkspaceAPI.createFolder,
    onSuccess: () => refreshTree(),
  });

  const renameNode = useMutation({
    mutationFn: (args: { id: string; name: string }) => WorkspaceAPI.renameNode(args.id, args.name),
    onSuccess: () => refreshTree(),
  });

  const deleteNode = useMutation({
    mutationFn: (id: string) => WorkspaceAPI.deleteNode(id),
    onSuccess: () => refreshTree(),
  });

  const saveFile = useMutation({
    mutationFn: (args: { nodeId: string; content: string; summary?: string }) => WorkspaceAPI.updateFile(args.nodeId, { content: args.content, summary: args.summary }),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: workspaceKeys.file(variables.nodeId) });
    },
  });

  const approveStaging = useMutation({
    mutationFn: (id: string) => WorkspaceAPI.approveStaging(id),
    onSuccess: () => {
      refreshTree();
      refreshStaging();
    },
  });

  const rejectStaging = useMutation({
    mutationFn: (args: { id: string; reason: string }) => WorkspaceAPI.rejectStaging(args.id, args.reason),
    onSuccess: () => refreshStaging(),
  });

  return { createFolder, renameNode, deleteNode, saveFile, approveStaging, rejectStaging, refreshTree };
}
