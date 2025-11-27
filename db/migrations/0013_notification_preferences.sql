-- 0008_notification_preferences.sql
-- 用户通知偏好管理与通知历史

-- 1. 创建用户通知偏好表
CREATE TABLE IF NOT EXISTS notification_preferences (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    
    -- 渠道启用状态
    enabled_channels JSONB DEFAULT '["websocket","email"]'::JSONB,
    
    -- 分类订阅设置
    category_preferences JSONB DEFAULT '{}'::JSONB,
    
    -- 邮件偏好
    email_address VARCHAR(255),
    email_frequency VARCHAR(20) DEFAULT 'realtime',
    digest_enabled BOOLEAN DEFAULT FALSE,
    
    -- 静音时段
    quiet_hours_start VARCHAR(5),  -- HH:MM 格式
    quiet_hours_end VARCHAR(5),    -- HH:MM 格式
    quiet_hours_enabled BOOLEAN DEFAULT FALSE,
    
    -- Webhook配置
    webhook_url VARCHAR(500),
    webhook_headers JSONB,
    
    -- 通知过滤规则
    filter_rules JSONB,
    
    -- 时区和语言
    timezone VARCHAR(50) DEFAULT 'UTC',
    language VARCHAR(10) DEFAULT 'zh-CN',
    
    -- 审计字段
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    
    -- 唯一约束：同一租户下用户ID唯一
    CONSTRAINT uk_notification_prefs_tenant_user UNIQUE (tenant_id, user_id)
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_notification_prefs_tenant_user ON notification_preferences(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_notification_prefs_user ON notification_preferences(user_id);

-- 2. 创建租户通知配置表
CREATE TABLE IF NOT EXISTS tenant_notification_configs (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL UNIQUE,
    
    -- 可用的通知渠道
    available_channels JSONB DEFAULT '["websocket","email","webhook"]'::JSONB,
    
    -- 默认邮件配置
    default_from_email VARCHAR(255),
    default_from_name VARCHAR(255),
    
    -- SMTP配置
    smtp_host VARCHAR(255),
    smtp_port INTEGER,
    smtp_username VARCHAR(255),
    smtp_password VARCHAR(500),  -- 应加密存储
    use_tls BOOLEAN DEFAULT TRUE,
    
    -- 默认Webhook配置
    default_webhook_url VARCHAR(500),
    
    -- 通知限流配置
    rate_limit_enabled BOOLEAN DEFAULT TRUE,
    max_notifications_per_hour INTEGER DEFAULT 100,
    
    -- 通知保留策略
    retention_days INTEGER DEFAULT 30,
    
    -- 审计字段
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_tenant_notif_config_tenant ON tenant_notification_configs(tenant_id);

-- 3. 创建通知历史表
CREATE TABLE IF NOT EXISTS notification_history (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    
    -- 通知类型
    channel VARCHAR(20) NOT NULL,
    category VARCHAR(50),
    
    -- 通知内容
    subject VARCHAR(500),
    body TEXT,
    data JSONB,
    
    -- 发送状态
    status VARCHAR(20) DEFAULT 'pending',  -- pending, sent, failed, skipped
    sent_at TIMESTAMPTZ,
    failed_reason TEXT,
    attempts INTEGER DEFAULT 0,
    
    -- 关联信息
    related_type VARCHAR(50),  -- approval, workflow, agent等
    related_id VARCHAR(255),
    
    -- 审计字段
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 创建索引（优化查询性能）
CREATE INDEX IF NOT EXISTS idx_notif_history_tenant_time ON notification_history(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notif_history_user ON notification_history(user_id);
CREATE INDEX IF NOT EXISTS idx_notif_history_category ON notification_history(category);
CREATE INDEX IF NOT EXISTS idx_notif_history_related ON notification_history(related_id);
CREATE INDEX IF NOT EXISTS idx_notif_history_status ON notification_history(status);

-- 4. 添加注释说明
COMMENT ON TABLE notification_preferences IS '用户通知偏好设置';
COMMENT ON COLUMN notification_preferences.enabled_channels IS '启用的通知渠道（websocket, email, webhook, sms, push）';
COMMENT ON COLUMN notification_preferences.category_preferences IS '分类订阅设置（approval, workflow, system, security, agent, quota）';
COMMENT ON COLUMN notification_preferences.email_frequency IS '邮件发送频率（realtime, hourly, daily, weekly, never）';
COMMENT ON COLUMN notification_preferences.quiet_hours_start IS '静音时段开始时间（HH:MM格式）';
COMMENT ON COLUMN notification_preferences.quiet_hours_end IS '静音时段结束时间（HH:MM格式）';
COMMENT ON COLUMN notification_preferences.filter_rules IS '通知过滤规则（关键词、优先级等）';

COMMENT ON TABLE tenant_notification_configs IS '租户级别通知配置';
COMMENT ON COLUMN tenant_notification_configs.available_channels IS '租户可用的通知渠道';
COMMENT ON COLUMN tenant_notification_configs.rate_limit_enabled IS '是否启用通知限流（防止通知轰炸）';
COMMENT ON COLUMN tenant_notification_configs.max_notifications_per_hour IS '每小时最大通知数量';
COMMENT ON COLUMN tenant_notification_configs.retention_days IS '通知历史保留天数';

COMMENT ON TABLE notification_history IS '通知历史记录（用于审计和重发）';
COMMENT ON COLUMN notification_history.status IS '发送状态（pending-待发送, sent-已发送, failed-失败, skipped-跳过）';
COMMENT ON COLUMN notification_history.attempts IS '发送尝试次数';
COMMENT ON COLUMN notification_history.related_type IS '关联对象类型（approval-审批, workflow-工作流, agent-智能体等）';
COMMENT ON COLUMN notification_history.related_id IS '关联对象ID';

-- 5. 插入默认数据（可选）
-- 为已存在的租户创建默认通知配置
INSERT INTO tenant_notification_configs (tenant_id, available_channels, rate_limit_enabled, max_notifications_per_hour, retention_days)
SELECT 
    id AS tenant_id,
    '["websocket","email","webhook"]'::JSONB AS available_channels,
    TRUE AS rate_limit_enabled,
    100 AS max_notifications_per_hour,
    30 AS retention_days
FROM tenants
WHERE NOT EXISTS (
    SELECT 1 FROM tenant_notification_configs tnc WHERE tnc.tenant_id = tenants.id
);
