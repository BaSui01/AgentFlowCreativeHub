import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { WorkspaceAPI } from './api';
import { WorkspaceFileDetailDTO, WorkspaceNodeDTO, WorkspaceStagingFileDTO, CommandRequestDTO } from './types';

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
		queryFn: () => WorkspaceAPI.listStaging(),
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
		mutationFn: (args: { nodeId: string; content: string; summary?: string; versionId?: string }) => WorkspaceAPI.saveFile({
			nodeId: args.nodeId,
			content: args.content,
			summary: args.summary,
			versionId: args.versionId,
		}),
		onSuccess: (_, variables) => {
			queryClient.invalidateQueries({ queryKey: workspaceKeys.file(variables.nodeId) });
		},
	});

	const approveStaging = useMutation({
		mutationFn: (args: { id: string; reviewToken: string }) =>
			WorkspaceAPI.reviewStaging(args.id, { action: 'approve', reviewToken: args.reviewToken }),
		onSuccess: () => {
			refreshTree();
			refreshStaging();
		},
	});

	const rejectStaging = useMutation({
		mutationFn: (args: { id: string; reviewToken: string; reason: string }) =>
			WorkspaceAPI.reviewStaging(args.id, {
				action: 'reject',
				reviewToken: args.reviewToken,
				reason: args.reason,
			}),
		onSuccess: () => refreshStaging(),
	});

	const requestChanges = useMutation({
		mutationFn: (args: { id: string; reviewToken: string; reason: string }) =>
			WorkspaceAPI.reviewStaging(args.id, {
				action: 'request_changes',
				reviewToken: args.reviewToken,
				reason: args.reason,
			}),
		onSuccess: () => refreshStaging(),
	});

	return { createFolder, renameNode, deleteNode, saveFile, approveStaging, rejectStaging, requestChanges, refreshTree };
}

export function useCommandStatus(commandId?: string) {
  const [state, setState] = useState<{ id?: string; data?: CommandRequestDTO; error?: unknown }>({});

  useEffect(() => {
    if (!commandId) {
      return undefined;
    }

    let cancelled = false;
    let timer: ReturnType<typeof setInterval> | undefined;

    const fetchStatus = async () => {
      try {
        if (cancelled) return;
        const result = await WorkspaceAPI.getCommandById(commandId);
        if (cancelled) return;
        setState({ id: commandId, data: result, error: undefined });
        if (!result || result.status === 'completed' || result.status === 'failed') {
          if (timer) {
            clearInterval(timer);
            timer = undefined;
          }
        }
      } catch (err) {
        if (cancelled) return;
        setState({ id: commandId, data: undefined, error: err });
      }
    };

    fetchStatus();
    timer = setInterval(fetchStatus, 2000);

    return () => {
      cancelled = true;
      if (timer) {
        clearInterval(timer);
      }
    };
  }, [commandId]);

  const isSameId = state.id === commandId;
  const command = isSameId ? state.data : undefined;
  const error = isSameId ? state.error : undefined;
  const isLoading = Boolean(commandId && (!isSameId || (!command && !error)));

  return { command, isLoading, error };
}
