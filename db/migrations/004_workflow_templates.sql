-- ========================================
-- 工作流模板和 Agent 能力表
-- ========================================

-- 工作流模板表
CREATE TABLE IF NOT EXISTS workflow_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID,
    
    -- 模板信息
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    
    -- 模板定义
    definition JSONB NOT NULL,
    
    -- 可见性
    is_system BOOLEAN DEFAULT FALSE,
    is_public BOOLEAN DEFAULT FALSE,
    
    -- 使用统计
    usage_count INTEGER DEFAULT 0,
    
    -- 创建人
    created_by VARCHAR(100),
    
    -- 时间戳
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- 索引
CREATE INDEX idx_workflow_templates_tenant ON workflow_templates(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_workflow_templates_category ON workflow_templates(category) WHERE deleted_at IS NULL;
CREATE INDEX idx_workflow_templates_system ON workflow_templates(is_system) WHERE deleted_at IS NULL;

-- 注释
COMMENT ON TABLE workflow_templates IS '工作流模板';
COMMENT ON COLUMN workflow_templates.is_system IS '是否系统预设模板';
COMMENT ON COLUMN workflow_templates.is_public IS '是否公开（租户间共享）';

-- ========================================
-- Agent 能力表
-- ========================================

CREATE TABLE IF NOT EXISTS agent_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Agent 信息
    agent_type VARCHAR(100) NOT NULL,
    role VARCHAR(100) NOT NULL,
    
    -- 能力描述
    name VARCHAR(255) NOT NULL,
    description TEXT,
    input_fields TEXT[],
    output_fields TEXT[],
    
    -- System Prompt 模板
    system_prompt_template TEXT,
    
    -- 推荐配置
    recommended_config JSONB,
    
    -- 是否启用
    enabled BOOLEAN DEFAULT TRUE,
    
    -- 时间戳
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 唯一索引（一个 Agent 类型的每个角色只能有一条记录）
CREATE UNIQUE INDEX idx_agent_capabilities_type_role ON agent_capabilities(agent_type, role);

-- 注释
COMMENT ON TABLE agent_capabilities IS 'Agent 能力定义';
COMMENT ON COLUMN agent_capabilities.role IS 'Agent 角色（如 outline_reviewer、content_reviewer）';
COMMENT ON COLUMN agent_capabilities.input_fields IS '输入字段列表';
COMMENT ON COLUMN agent_capabilities.output_fields IS '输出字段列表';

-- ========================================
-- 更新触发器
-- ========================================

-- 工作流模板更新触发器
CREATE OR REPLACE FUNCTION update_workflow_template_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_workflow_templates_updated_at
    BEFORE UPDATE ON workflow_templates
    FOR EACH ROW
    EXECUTE FUNCTION update_workflow_template_timestamp();

-- Agent 能力更新触发器
CREATE OR REPLACE FUNCTION update_agent_capability_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_agent_capabilities_updated_at
    BEFORE UPDATE ON agent_capabilities
    FOR EACH ROW
    EXECUTE FUNCTION update_agent_capability_timestamp();
