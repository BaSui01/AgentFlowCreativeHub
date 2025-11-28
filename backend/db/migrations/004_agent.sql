-- ============================================================
-- 004_agent.sql - Agent 模块
-- ============================================================

-- ============================================================
-- 1. Agent 配置表
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- Agent 信息
    agent_type VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    role VARCHAR(100),
    
    -- 模型配置（主/备用）
    primary_model_id UUID REFERENCES models(id),
    secondary_model_id UUID REFERENCES models(id),
    fallback_strategy VARCHAR(32) NOT NULL DEFAULT 'auto',
    fallback_timeout_ms INTEGER DEFAULT 0,
    last_fallback_at TIMESTAMPTZ,
    
    -- 任务专用模型（按任务类型路由）
    tool_model_id UUID REFERENCES models(id),        -- 工具调用：便宜快速的模型
    creative_model_id UUID REFERENCES models(id),    -- 创作任务：擅长写作的模型
    analysis_model_id UUID REFERENCES models(id),    -- 分析任务：强逻辑推理的模型
    summary_model_id UUID REFERENCES models(id),     -- 摘要任务：性价比高的模型
    
    -- 灵活模型路由配置（支持自定义任务类型）
    -- 示例: {"code_gen": "model-uuid", "translation": "model-uuid"}
    model_routing JSONB DEFAULT '{}'::JSONB,
    
    -- System Prompt
    system_prompt TEXT,
    
    -- 参数配置
    temperature DECIMAL(3, 2) DEFAULT 0.7,
    max_tokens INT DEFAULT 2048,
    top_p DECIMAL(3, 2) DEFAULT 1.0,
    
    -- 工具配置
    tools JSONB DEFAULT '[]'::JSONB,
    
    -- 知识库配置
    knowledge_base_ids UUID[] DEFAULT '{}',
    
    -- 状态
    status VARCHAR(32) DEFAULT 'active',
    is_active BOOLEAN DEFAULT TRUE,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_configs_tenant ON agent_configs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_configs_type ON agent_configs(agent_type);
CREATE INDEX IF NOT EXISTS idx_agent_configs_primary_model ON agent_configs(primary_model_id);
CREATE INDEX IF NOT EXISTS idx_agent_configs_secondary_model ON agent_configs(secondary_model_id);
CREATE INDEX IF NOT EXISTS idx_agent_configs_tool_model ON agent_configs(tool_model_id);
CREATE INDEX IF NOT EXISTS idx_agent_configs_creative_model ON agent_configs(creative_model_id);

-- ============================================================
-- 2. Agent 能力表
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_type VARCHAR(100) NOT NULL,
    role VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    input_fields TEXT[],
    output_fields TEXT[],
    system_prompt_template TEXT,
    recommended_config JSONB DEFAULT '{}'::JSONB,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_capabilities_type_role ON agent_capabilities(agent_type, role);

-- ============================================================
-- 3. Agent 配置版本表
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_config_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_config_id UUID NOT NULL REFERENCES agent_configs(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    version INT NOT NULL,
    snapshot JSONB NOT NULL,
    change_log TEXT,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_config_versions_config ON agent_config_versions(agent_config_id);
CREATE INDEX IF NOT EXISTS idx_agent_config_versions_tenant ON agent_config_versions(tenant_id);

-- ============================================================
-- 4. Agent 产出物表
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL,
    agent_name VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    node_id UUID,
    
    -- 文件信息
    artifact_type VARCHAR(50) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(1024) NOT NULL,
    file_size BIGINT DEFAULT 0,
    content_hash VARCHAR(64),
    
    -- 内容
    summary TEXT,
    
    -- 来源
    task_type VARCHAR(50),
    tool_name VARCHAR(100),
    sequence INT DEFAULT 1,
    
    -- 状态
    status VARCHAR(50) DEFAULT 'created',
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_artifact_tenant ON agent_artifacts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_artifact_agent ON agent_artifacts(agent_id);
CREATE INDEX IF NOT EXISTS idx_artifact_session ON agent_artifacts(session_id);
CREATE INDEX IF NOT EXISTS idx_artifact_node ON agent_artifacts(node_id);

-- ============================================================
-- 5. Agent 专属工作空间表
-- ============================================================
CREATE TABLE IF NOT EXISTS agent_workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL,
    agent_name VARCHAR(255) NOT NULL,
    
    -- 目录节点
    root_node_id UUID,
    outputs_node_id UUID,
    drafts_node_id UUID,
    logs_node_id UUID,
    
    -- 统计
    artifact_count INT DEFAULT 0,
    total_file_size BIGINT DEFAULT 0,
    last_activity_at TIMESTAMPTZ,
    
    -- 配置
    settings JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_workspace_tenant ON agent_workspaces(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_workspace_unique ON agent_workspaces(tenant_id, agent_id);

-- ============================================================
-- 6. 会话专属工作空间表
-- ============================================================
CREATE TABLE IF NOT EXISTS session_workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id VARCHAR(255) NOT NULL,
    
    -- 目录节点
    root_node_id UUID,
    context_node_id UUID,
    artifacts_node_id UUID,
    history_node_id UUID,
    
    -- 统计
    artifact_count INT DEFAULT 0,
    agent_ids JSONB DEFAULT '[]'::JSONB,
    
    -- 配置
    settings JSONB DEFAULT '{}'::JSONB,
    expires_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_session_workspace_tenant ON session_workspaces(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_session_workspace_unique ON session_workspaces(tenant_id, session_id);

COMMENT ON TABLE agent_configs IS 'Agent 配置表';
COMMENT ON TABLE agent_capabilities IS 'Agent 能力定义表';
COMMENT ON TABLE agent_artifacts IS 'Agent 产出物记录表';
COMMENT ON TABLE agent_workspaces IS 'Agent 专属工作空间表';
COMMENT ON TABLE session_workspaces IS '会话专属工作空间表';
