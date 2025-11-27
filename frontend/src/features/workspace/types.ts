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

export type StagingStatus =
  | 'drafted'
  | 'awaiting_review'
  | 'awaiting_secondary_review'
  | 'approved_pending_archive'
  | 'archived'
  | 'changes_requested'
  | 'rejected'
  | 'failed';

export interface WorkspaceStagingFileDTO {
	id: string;
	fileType: string;
	suggestedName: string;
	suggestedFolder: string;
	suggestedPath: string;
	summary?: string;
	status: StagingStatus;
	createdAt: string;
	sourceAgentName?: string;
	metadata?: Record<string, unknown>;
	reviewToken?: string;
	secondaryReviewToken?: string;
	requiresSecondary?: boolean;
	reviewer_id?: string;
	secondaryReviewerId?: string;
	slaExpiresAt?: string;
	resubmitCount?: number;
	auditTrail?: unknown[];
}

export interface AttachContextPayload {
  agentId: string;
  sessionId?: string;
  nodeIds: string[];
  mentions: string[];
  commands: string[];
  notes?: string;
}

export type CommandStatus = 'queued' | 'running' | 'completed' | 'failed';

export interface CommandRequestDTO {
  id: string;
  agentId: string;
  commandType?: string;
  status: CommandStatus;
  contextSnapshot?: string;
  contextRevisionId?: string;
  notes?: string;
  resultPreview?: string;
  failureReason?: string;
  latencyMs?: number;
  tokenCost?: number;
  traceId?: string;
  deadlineAt?: string | null;
  createdAt?: string;
  updatedAt?: string;
  queuePosition?: number;
}

export interface ExecuteCommandPayload {
  agentId: string;
  content: string;
  commandType?: string;
  contextNodeIds: string[];
  sessionId?: string;
  notes?: string;
  deadlineMs?: number;
}

export interface ExecuteCommandResponse {
  request: CommandRequestDTO;
  new: boolean;
}

export interface CommandListResult {
	items: CommandRequestDTO[];
	total: number;
	page: number;
	pageSize: number;
}
