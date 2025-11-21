-- Workspace 节点表
CREATE TABLE IF NOT EXISTS workspace_nodes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    parent_id UUID NULL REFERENCES workspace_nodes(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL,
    node_path VARCHAR(1024) NOT NULL,
    category VARCHAR(50),
    sort_order INT DEFAULT 0,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_nodes_path ON workspace_nodes(tenant_id, node_path) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workspace_nodes_parent ON workspace_nodes(tenant_id, parent_id) WHERE deleted_at IS NULL;

-- Workspace 文件
CREATE TABLE IF NOT EXISTS workspace_files (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    node_id UUID NOT NULL REFERENCES workspace_nodes(id) ON DELETE CASCADE,
    latest_version_id UUID,
    category VARCHAR(50),
    auto_tags JSONB DEFAULT '[]'::jsonb,
    review_status VARCHAR(50) NOT NULL DEFAULT 'published',
    approver_id UUID,
    approved_at TIMESTAMPTZ,
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_file_node ON workspace_files(node_id);

-- Workspace 文件版本
CREATE TABLE IF NOT EXISTS workspace_file_versions (
    id UUID PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES workspace_files(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    content TEXT,
    summary TEXT,
    agent_id UUID,
    tool_name VARCHAR(100),
    metadata JSONB DEFAULT '{}'::jsonb,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_versions_file ON workspace_file_versions(file_id);

-- 暂存区
CREATE TABLE IF NOT EXISTS workspace_staging_files (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    suggested_name VARCHAR(255) NOT NULL,
    suggested_folder VARCHAR(255) NOT NULL,
    suggested_path VARCHAR(1024) NOT NULL,
    content TEXT NOT NULL,
    summary TEXT,
    source_agent_id UUID,
    source_agent_name VARCHAR(255),
    source_command VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    reviewer_id UUID,
    reviewer_name VARCHAR(255),
    reviewed_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_staging_tenant_status ON workspace_staging_files(tenant_id, status);

-- 命令上下文绑定
CREATE TABLE IF NOT EXISTS workspace_context_links (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    agent_id UUID,
    session_id VARCHAR(255) NOT NULL,
    mentions JSONB DEFAULT '[]'::jsonb,
    commands JSONB DEFAULT '[]'::jsonb,
    node_ids JSONB DEFAULT '[]'::jsonb,
    notes TEXT,
    snapshot TEXT,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_context_tenant ON workspace_context_links(tenant_id);
