-- ============================================================
-- 008_notification.sql - 通知模块
-- ============================================================

-- ============================================================
-- 1. 用户通知偏好表
-- ============================================================
CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 渠道启用状态
    enabled_channels JSONB DEFAULT '["websocket","email"]'::JSONB,
    
    -- 分类订阅设置
    category_preferences JSONB DEFAULT '{}'::JSONB,
    
    -- 邮件偏好
    email_address VARCHAR(255),
    email_frequency VARCHAR(20) DEFAULT 'realtime',
    digest_enabled BOOLEAN DEFAULT FALSE,
    
    -- 静音时段
    quiet_hours_start VARCHAR(5),
    quiet_hours_end VARCHAR(5),
    quiet_hours_enabled BOOLEAN DEFAULT FALSE,
    
    -- Webhook 配置
    webhook_url VARCHAR(500),
    webhook_headers JSONB DEFAULT '{}'::JSONB,
    
    -- 通知过滤规则
    filter_rules JSONB DEFAULT '{}'::JSONB,
    
    -- 时区和语言
    timezone VARCHAR(50) DEFAULT 'Asia/Shanghai',
    language VARCHAR(10) DEFAULT 'zh-CN',
    
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uk_notification_prefs_tenant_user UNIQUE (tenant_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_notification_prefs_tenant_user ON notification_preferences(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_notification_prefs_user ON notification_preferences(user_id);

-- ============================================================
-- 2. 租户通知配置表
-- ============================================================
CREATE TABLE IF NOT EXISTS tenant_notification_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- 可用的通知渠道
    available_channels JSONB DEFAULT '["websocket","email","webhook"]'::JSONB,
    
    -- 默认邮件配置
    default_from_email VARCHAR(255),
    default_from_name VARCHAR(255),
    
    -- SMTP 配置
    smtp_host VARCHAR(255),
    smtp_port INTEGER,
    smtp_username VARCHAR(255),
    smtp_password VARCHAR(500),
    use_tls BOOLEAN DEFAULT TRUE,
    
    -- 默认 Webhook 配置
    default_webhook_url VARCHAR(500),
    
    -- 通知限流配置
    rate_limit_enabled BOOLEAN DEFAULT TRUE,
    max_notifications_per_hour INTEGER DEFAULT 100,
    
    -- 通知保留策略
    retention_days INTEGER DEFAULT 30,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_notif_config_tenant ON tenant_notification_configs(tenant_id);

-- ============================================================
-- 3. 通知历史表
-- ============================================================
CREATE TABLE IF NOT EXISTS notification_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 通知类型
    channel VARCHAR(20) NOT NULL,
    category VARCHAR(50),
    
    -- 通知内容
    subject VARCHAR(500),
    body TEXT,
    data JSONB DEFAULT '{}'::JSONB,
    
    -- 发送状态
    status VARCHAR(20) DEFAULT 'pending',
    sent_at TIMESTAMPTZ,
    failed_reason TEXT,
    attempts INTEGER DEFAULT 0,
    
    -- 关联信息
    related_type VARCHAR(50),
    related_id VARCHAR(255),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notif_history_tenant_time ON notification_history(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notif_history_user ON notification_history(user_id);
CREATE INDEX IF NOT EXISTS idx_notif_history_category ON notification_history(category);
CREATE INDEX IF NOT EXISTS idx_notif_history_related ON notification_history(related_id);
CREATE INDEX IF NOT EXISTS idx_notif_history_status ON notification_history(status);

-- ============================================================
-- 4. 用户消息表
-- ============================================================
CREATE TABLE IF NOT EXISTS user_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 消息内容
    title VARCHAR(200) NOT NULL,
    content TEXT,
    category VARCHAR(50),
    priority VARCHAR(20) DEFAULT 'normal',
    
    -- 状态
    status VARCHAR(20) DEFAULT 'unread',
    read_at TIMESTAMPTZ,
    
    -- 关联
    related_type VARCHAR(50),
    related_id VARCHAR(255),
    action_url VARCHAR(500),
    
    -- 发送者
    sender_id UUID,
    sender_name VARCHAR(100),
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    expires_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_messages_tenant ON user_messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_messages_user_status ON user_messages(user_id, status);
CREATE INDEX IF NOT EXISTS idx_user_messages_category ON user_messages(category);
CREATE INDEX IF NOT EXISTS idx_user_messages_expires ON user_messages(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_messages_deleted_at ON user_messages(deleted_at);

COMMENT ON TABLE notification_preferences IS '用户通知偏好设置';
COMMENT ON TABLE tenant_notification_configs IS '租户级别通知配置';
COMMENT ON TABLE notification_history IS '通知历史记录';
COMMENT ON TABLE user_messages IS '用户消息表';
