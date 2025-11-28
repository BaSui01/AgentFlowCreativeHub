-- 用户消息表（消息已读状态）
CREATE TABLE IF NOT EXISTS user_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    
    title VARCHAR(200) NOT NULL,
    content TEXT,
    category VARCHAR(50),
    priority VARCHAR(20) DEFAULT 'normal',
    
    status VARCHAR(20) DEFAULT 'unread',
    read_at TIMESTAMP,
    
    related_type VARCHAR(50),
    related_id VARCHAR(255),
    action_url VARCHAR(500),
    
    sender_id UUID,
    sender_name VARCHAR(100),
    
    metadata JSONB,
    expires_at TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_user_messages_tenant ON user_messages(tenant_id);
CREATE INDEX idx_user_messages_user_status ON user_messages(user_id, status);
CREATE INDEX idx_user_messages_category ON user_messages(category);
CREATE INDEX idx_user_messages_expires ON user_messages(expires_at);
CREATE INDEX idx_user_messages_deleted_at ON user_messages(deleted_at);

COMMENT ON TABLE user_messages IS '用户消息表：支持已读/未读状态管理';
COMMENT ON COLUMN user_messages.status IS '消息状态：unread, read';
COMMENT ON COLUMN user_messages.priority IS '优先级：low, normal, high, urgent';
COMMENT ON COLUMN user_messages.category IS '消息分类：approval, workflow, system, security, agent, quota';

-- 工作空间模板表
CREATE TABLE IF NOT EXISTS workspace_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID,
    
    name VARCHAR(100) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    
    structure JSONB NOT NULL,
    
    preview_url VARCHAR(500),
    tags TEXT,
    is_builtin BOOLEAN DEFAULT FALSE,
    is_public BOOLEAN DEFAULT FALSE,
    use_count INT DEFAULT 0,
    
    created_by UUID,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_workspace_templates_tenant ON workspace_templates(tenant_id);
CREATE INDEX idx_workspace_templates_type ON workspace_templates(type);
CREATE INDEX idx_workspace_templates_deleted_at ON workspace_templates(deleted_at);

COMMENT ON TABLE workspace_templates IS '工作空间模板表：预置项目结构模板';
COMMENT ON COLUMN workspace_templates.type IS '模板类型：novel, script, article, project, custom';
COMMENT ON COLUMN workspace_templates.structure IS 'JSON格式的模板结构定义';
COMMENT ON COLUMN workspace_templates.is_builtin IS '是否系统内置模板';
COMMENT ON COLUMN workspace_templates.is_public IS '是否公开（可被其他租户使用）';
COMMENT ON COLUMN workspace_templates.use_count IS '使用次数统计';
