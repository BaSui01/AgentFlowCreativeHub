-- ============================================================
-- 006_workflow.sql - 工作流模块
-- ============================================================

-- ============================================================
-- 1. 工作流模板表
-- ============================================================
CREATE TABLE IF NOT EXISTS workflow_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    
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
    
    created_by VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workflow_templates_tenant ON workflow_templates(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflow_templates_category ON workflow_templates(category) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflow_templates_system ON workflow_templates(is_system) WHERE deleted_at IS NULL;

-- ============================================================
-- 2. 工作流定义表
-- ============================================================
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    owner_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- 工作流信息
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    
    -- 定义
    definition JSONB NOT NULL,
    variables JSONB DEFAULT '{}'::JSONB,
    
    -- 状态
    status VARCHAR(50) DEFAULT 'draft',
    version INT DEFAULT 1,
    
    -- 调度
    schedule VARCHAR(100),
    next_run_at TIMESTAMPTZ,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workflows_tenant ON workflows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflows_owner ON workflows(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);

-- ============================================================
-- 3. 工作流执行表
-- ============================================================
CREATE TABLE IF NOT EXISTS workflow_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- 执行信息
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    current_step VARCHAR(100),
    
    -- 输入输出
    input JSONB DEFAULT '{}'::JSONB,
    output JSONB DEFAULT '{}'::JSONB,
    
    -- 时间
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    -- 错误
    error_code VARCHAR(64),
    error_message TEXT,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflow_executions_tenant ON workflow_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow ON workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(status);

-- ============================================================
-- 4. 工作流任务表
-- ============================================================
CREATE TABLE IF NOT EXISTS workflow_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    execution_id UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    
    -- 任务信息
    step_id VARCHAR(100) NOT NULL,
    step_type VARCHAR(50) NOT NULL,
    step_name VARCHAR(255),
    
    -- 状态
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    
    -- 输入输出
    input JSONB DEFAULT '{}'::JSONB,
    output JSONB DEFAULT '{}'::JSONB,
    
    -- 时间
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    -- 错误
    error_message TEXT,
    
    -- 重试
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflow_tasks_execution ON workflow_tasks(execution_id);
CREATE INDEX IF NOT EXISTS idx_workflow_tasks_status ON workflow_tasks(status);

-- ============================================================
-- 5. 审批请求表
-- ============================================================
CREATE TABLE IF NOT EXISTS approval_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    execution_id UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    workflow_id UUID REFERENCES workflows(id) ON DELETE SET NULL,
    
    -- 审批信息
    step_id VARCHAR(100) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- 审批人
    assignee_ids UUID[] DEFAULT '{}',
    assignee_type VARCHAR(50) DEFAULT 'user',
    
    -- 状态
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    
    -- 审批结果
    approved_by UUID,
    approved_at TIMESTAMPTZ,
    rejected_by UUID,
    rejected_at TIMESTAMPTZ,
    comment TEXT,
    
    -- 通知
    notify_targets JSONB DEFAULT '{}'::JSONB,
    notification_attempts INT NOT NULL DEFAULT 0,
    last_notified_at TIMESTAMPTZ,
    last_notification_error TEXT,
    
    -- 过期
    expires_at TIMESTAMPTZ,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_approval_requests_tenant ON approval_requests(tenant_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_execution ON approval_requests(execution_id);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests(status);

-- ============================================================
-- 6. 自动化日志表
-- ============================================================
CREATE TABLE IF NOT EXISTS automation_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    execution_id UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    step_id VARCHAR(100),
    
    -- 日志信息
    level VARCHAR(20) NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    
    -- 详情
    details JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_automation_logs_execution ON automation_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_automation_logs_tenant ON automation_logs(tenant_id, created_at DESC);

-- ============================================================
-- 7. 审批规则表
-- ============================================================
CREATE TABLE IF NOT EXISTS approval_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workflow_id UUID REFERENCES workflows(id) ON DELETE SET NULL,
    
    -- 规则信息
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- 条件
    conditions JSONB NOT NULL,
    
    -- 动作
    actions JSONB NOT NULL,
    
    -- 优先级
    priority INT DEFAULT 0,
    
    -- 状态
    is_active BOOLEAN DEFAULT TRUE,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_approval_rules_tenant ON approval_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_approval_rules_workflow ON approval_rules(workflow_id);

COMMENT ON TABLE workflow_templates IS '工作流模板表';
COMMENT ON TABLE workflows IS '工作流定义表';
COMMENT ON TABLE workflow_executions IS '工作流执行记录表';
COMMENT ON TABLE workflow_tasks IS '工作流任务表';
COMMENT ON TABLE approval_requests IS '审批请求表';
COMMENT ON TABLE automation_logs IS '自动化日志表';
