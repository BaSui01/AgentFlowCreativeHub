-- 智能体产出物记录表
CREATE TABLE IF NOT EXISTS agent_artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    agent_name VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    node_id UUID,
    artifact_type VARCHAR(50) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(1024) NOT NULL,
    file_size BIGINT DEFAULT 0,
    content_hash VARCHAR(64),
    summary TEXT,
    task_type VARCHAR(50),
    tool_name VARCHAR(100),
    sequence INT DEFAULT 1,
    status VARCHAR(50) DEFAULT 'created',
    metadata JSONB,
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifact_tenant ON agent_artifacts(tenant_id);
CREATE INDEX idx_artifact_agent ON agent_artifacts(agent_id);
CREATE INDEX idx_artifact_session ON agent_artifacts(session_id);
CREATE INDEX idx_artifact_node ON agent_artifacts(node_id);

-- 智能体专属工作空间表
CREATE TABLE IF NOT EXISTS agent_workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    agent_name VARCHAR(255) NOT NULL,
    root_node_id UUID,
    outputs_node_id UUID,
    drafts_node_id UUID,
    logs_node_id UUID,
    artifact_count INT DEFAULT 0,
    total_file_size BIGINT DEFAULT 0,
    last_activity_at TIMESTAMP,
    settings JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_workspace_tenant ON agent_workspaces(tenant_id);
CREATE UNIQUE INDEX idx_agent_workspace_unique ON agent_workspaces(tenant_id, agent_id);

-- 会话专属工作空间表
CREATE TABLE IF NOT EXISTS session_workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    session_id VARCHAR(255) NOT NULL,
    root_node_id UUID,
    context_node_id UUID,
    artifacts_node_id UUID,
    history_node_id UUID,
    artifact_count INT DEFAULT 0,
    agent_ids JSONB,
    settings JSONB,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_workspace_tenant ON session_workspaces(tenant_id);
CREATE UNIQUE INDEX idx_session_workspace_unique ON session_workspaces(tenant_id, session_id);

-- 添加注释
COMMENT ON TABLE agent_artifacts IS '智能体产出物记录';
COMMENT ON TABLE agent_workspaces IS '智能体专属工作空间';
COMMENT ON TABLE session_workspaces IS '会话专属工作空间';
