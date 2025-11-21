export interface WorkspaceNodeDTO {
  id: string;
  name: string;
  type: 'folder' | 'file';
  nodePath: string;
  category?: string;
  metadata?: Record<string, unknown>;
  children?: WorkspaceNodeDTO[];
}

export interface WorkspaceFileVersionDTO {
  id: string;
  summary?: string;
  content?: string;
  createdAt?: string;
  toolName?: string;
}

export interface WorkspaceFileDTO {
  id?: string;
  nodeId: string;
  reviewStatus?: string;
  latestVersionId?: string;
}

export interface WorkspaceFileDetailDTO {
  node: WorkspaceNodeDTO;
  file?: WorkspaceFileDTO;
  version?: WorkspaceFileVersionDTO;
}

export interface WorkspaceStagingFileDTO {
  id: string;
  fileType: string;
  suggestedName: string;
  suggestedFolder: string;
  suggestedPath: string;
  summary?: string;
  status: 'pending' | 'approved' | 'rejected';
  createdAt: string;
  sourceAgentName?: string;
  metadata?: Record<string, unknown>;
}

export interface AttachContextPayload {
  agentId: string;
  sessionId?: string;
  nodeIds: string[];
  mentions: string[];
  commands: string[];
  notes?: string;
}
