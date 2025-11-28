-- ============================================================
-- 005_workspace.sql - 工作空间模块
-- ============================================================

-- ============================================================
-- 1. 工作空间节点表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES workspace_nodes(id) ON DELETE SET NULL,
    
    -- 节点信息
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL,
    node_path VARCHAR(1024) NOT NULL,
    category VARCHAR(50),
    sort_order INT DEFAULT 0,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    -- 操作人
    created_by UUID,
    updated_by UUID,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_nodes_path ON workspace_nodes(tenant_id, node_path) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workspace_nodes_parent ON workspace_nodes(tenant_id, parent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workspace_nodes_tenant ON workspace_nodes(tenant_id);

-- ============================================================
-- 2. 工作空间文件表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    node_id UUID NOT NULL REFERENCES workspace_nodes(id) ON DELETE CASCADE,
    latest_version_id UUID,
    
    -- 分类
    category VARCHAR(50),
    auto_tags JSONB DEFAULT '[]'::JSONB,
    
    -- 审核
    review_status VARCHAR(50) NOT NULL DEFAULT 'published',
    approver_id UUID,
    approved_at TIMESTAMPTZ,
    
    -- 操作人
    created_by UUID,
    updated_by UUID,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_file_node ON workspace_files(node_id);
CREATE INDEX IF NOT EXISTS idx_workspace_files_tenant ON workspace_files(tenant_id);

-- ============================================================
-- 3. 工作空间文件版本表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_file_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES workspace_files(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- 内容
    content TEXT,
    summary TEXT,
    
    -- 来源
    agent_id UUID,
    tool_name VARCHAR(100),
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_versions_file ON workspace_file_versions(file_id);
CREATE INDEX IF NOT EXISTS idx_workspace_versions_tenant ON workspace_file_versions(tenant_id);

-- ============================================================
-- 4. 暂存区文件表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_staging_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- 文件信息
    file_type VARCHAR(50) NOT NULL,
    suggested_name VARCHAR(255) NOT NULL,
    suggested_folder VARCHAR(255) NOT NULL,
    suggested_path VARCHAR(1024) NOT NULL,
    
    -- 内容
    content TEXT NOT NULL,
    summary TEXT,
    
    -- 来源
    source_agent_id UUID,
    source_agent_name VARCHAR(255),
    source_command VARCHAR(255),
    
    -- 审核
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    reviewer_id UUID,
    reviewer_name VARCHAR(255),
    reviewed_at TIMESTAMPTZ,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_staging_tenant_status ON workspace_staging_files(tenant_id, status);

-- ============================================================
-- 5. 命令上下文绑定表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_context_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id UUID,
    session_id VARCHAR(255) NOT NULL,
    
    -- 上下文
    mentions JSONB DEFAULT '[]'::JSONB,
    commands JSONB DEFAULT '[]'::JSONB,
    node_ids JSONB DEFAULT '[]'::JSONB,
    notes TEXT,
    snapshot TEXT,
    
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_context_tenant ON workspace_context_links(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workspace_context_session ON workspace_context_links(session_id);

-- ============================================================
-- 6. 文件上传记录表
-- ============================================================
CREATE TABLE IF NOT EXISTS file_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- 文件信息
    file_name VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100),
    file_size BIGINT DEFAULT 0,
    file_hash VARCHAR(64),
    
    -- 存储
    storage_path VARCHAR(1024) NOT NULL,
    storage_type VARCHAR(50) DEFAULT 'local',
    
    -- 关联
    related_type VARCHAR(50),
    related_id UUID,
    
    -- 状态
    status VARCHAR(50) DEFAULT 'uploaded',
    
    -- 操作人
    uploaded_by UUID,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_upload_tenant ON file_uploads(tenant_id);
CREATE INDEX IF NOT EXISTS idx_file_upload_related ON file_uploads(related_type, related_id);

-- ============================================================
-- 7. 工作空间模板表
-- ============================================================
CREATE TABLE IF NOT EXISTS workspace_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- 模板信息
    name VARCHAR(100) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    
    -- 结构定义
    structure JSONB NOT NULL,
    
    -- 展示
    preview_url VARCHAR(500),
    tags TEXT,
    
    -- 状态
    is_builtin BOOLEAN DEFAULT FALSE,
    is_public BOOLEAN DEFAULT FALSE,
    use_count INT DEFAULT 0,
    
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workspace_templates_tenant ON workspace_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workspace_templates_type ON workspace_templates(type);
CREATE INDEX IF NOT EXISTS idx_workspace_templates_deleted_at ON workspace_templates(deleted_at);

COMMENT ON TABLE workspace_nodes IS '工作空间节点表（文件/文件夹）';
COMMENT ON TABLE workspace_files IS '工作空间文件表';
COMMENT ON TABLE workspace_file_versions IS '工作空间文件版本表';
COMMENT ON TABLE workspace_staging_files IS '暂存区文件表';
COMMENT ON TABLE workspace_context_links IS '命令上下文绑定表';
COMMENT ON TABLE workspace_templates IS '工作空间模板表';
