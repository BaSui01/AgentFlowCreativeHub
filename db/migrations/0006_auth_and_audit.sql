-- 0006_auth_and_audit.sql
-- Sprint 5: 认证授权 + 审计系统数据库迁移
-- 创建日期: 2025-11-17

-- =====================================================
-- 1. 创建 sessions 表（用户会话管理）
-- =====================================================

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    refresh_token TEXT NOT NULL UNIQUE,
    provider VARCHAR(50) NOT NULL,  -- local, google, github, microsoft, oidc
    ip_address VARCHAR(100),
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP NOT NULL,
    is_revoked BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 创建索引
CREATE INDEX idx_session_user ON sessions(user_id);
CREATE INDEX idx_session_tenant ON sessions(tenant_id);
CREATE INDEX idx_session_expires ON sessions(expires_at);
CREATE INDEX idx_session_revoked ON sessions(is_revoked);
CREATE UNIQUE INDEX idx_session_refresh_token ON sessions(refresh_token);

-- 添加注释
COMMENT ON TABLE sessions IS '用户会话表';
COMMENT ON COLUMN sessions.id IS '会话 ID';
COMMENT ON COLUMN sessions.user_id IS '用户 ID';
COMMENT ON COLUMN sessions.tenant_id IS '租户 ID';
COMMENT ON COLUMN sessions.refresh_token IS '刷新令牌（唯一）';
COMMENT ON COLUMN sessions.provider IS '认证提供商：local（本地）、google、github、microsoft、oidc';
COMMENT ON COLUMN sessions.ip_address IS '客户端 IP 地址';
COMMENT ON COLUMN sessions.user_agent IS '客户端 User Agent';
COMMENT ON COLUMN sessions.expires_at IS '会话过期时间';
COMMENT ON COLUMN sessions.last_used_at IS '最后使用时间';
COMMENT ON COLUMN sessions.is_revoked IS '是否已撤销';

-- =====================================================
-- 2. 创建 audit_logs 表（审计日志）
-- =====================================================

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID,
    event_type VARCHAR(100) NOT NULL,          -- 事件类型：user.login, model.create 等
    event_category VARCHAR(50) NOT NULL,       -- 事件分类：auth, resource, system, data, security
    event_level VARCHAR(20) NOT NULL,          -- 事件级别：info, warning, error, critical
    description TEXT,                          -- 事件描述
    ip_address VARCHAR(100),                   -- 客户端 IP
    user_agent TEXT,                           -- User Agent
    request_path TEXT,                         -- 请求路径
    request_method VARCHAR(10),                -- 请求方法：GET, POST, PUT, DELETE
    status_code INT,                           -- HTTP 状态码
    metadata JSONB,                            -- 元数据（JSON 格式）
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 创建索引
CREATE INDEX idx_audit_tenant ON audit_logs(tenant_id);
CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_category ON audit_logs(event_category);
CREATE INDEX idx_audit_created_at ON audit_logs(created_at DESC);

-- 创建复合索引（常见查询场景）
CREATE INDEX idx_audit_tenant_user ON audit_logs(tenant_id, user_id);
CREATE INDEX idx_audit_tenant_event ON audit_logs(tenant_id, event_type);
CREATE INDEX idx_audit_tenant_created ON audit_logs(tenant_id, created_at DESC);

-- 添加注释
COMMENT ON TABLE audit_logs IS '审计日志表';
COMMENT ON COLUMN audit_logs.id IS '日志 ID';
COMMENT ON COLUMN audit_logs.tenant_id IS '租户 ID';
COMMENT ON COLUMN audit_logs.user_id IS '用户 ID';
COMMENT ON COLUMN audit_logs.event_type IS '事件类型（如：user.login, model.create）';
COMMENT ON COLUMN audit_logs.event_category IS '事件分类（auth, resource, system, data, security）';
COMMENT ON COLUMN audit_logs.event_level IS '事件级别（info, warning, error, critical）';
COMMENT ON COLUMN audit_logs.description IS '事件描述';
COMMENT ON COLUMN audit_logs.ip_address IS '客户端 IP 地址';
COMMENT ON COLUMN audit_logs.user_agent IS '客户端 User Agent';
COMMENT ON COLUMN audit_logs.request_path IS 'API 请求路径';
COMMENT ON COLUMN audit_logs.request_method IS 'HTTP 请求方法';
COMMENT ON COLUMN audit_logs.status_code IS 'HTTP 响应状态码';
COMMENT ON COLUMN audit_logs.metadata IS '元数据（JSON 格式，包含资源 ID、变更内容等）';

-- =====================================================
-- 3. 创建定时清理函数（可选）
-- =====================================================

-- 清理过期会话的函数
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    affected_rows INTEGER;
BEGIN
    UPDATE sessions
    SET is_revoked = TRUE, updated_at = CURRENT_TIMESTAMP
    WHERE expires_at < CURRENT_TIMESTAMP AND is_revoked = FALSE;
    
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    RETURN affected_rows;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_expired_sessions IS '清理过期会话（将 is_revoked 设为 true）';

-- 清理旧审计日志的函数（保留最近 90 天）
CREATE OR REPLACE FUNCTION cleanup_old_audit_logs(retention_days INTEGER DEFAULT 90)
RETURNS INTEGER AS $$
DECLARE
    affected_rows INTEGER;
BEGIN
    DELETE FROM audit_logs
    WHERE created_at < CURRENT_TIMESTAMP - (retention_days || ' days')::INTERVAL;
    
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    RETURN affected_rows;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_old_audit_logs IS '清理旧审计日志（默认保留 90 天）';

-- =====================================================
-- 4. 插入示例数据（可选）
-- =====================================================

-- 示例：插入管理员用户的会话（如果需要测试）
-- INSERT INTO sessions (user_id, tenant_id, refresh_token, provider, ip_address, expires_at, last_used_at)
-- VALUES (
--     '00000000-0000-0000-0000-000000000001',
--     '00000000-0000-0000-0000-000000000001',
--     'sample_refresh_token_for_testing',
--     'local',
--     '127.0.0.1',
--     CURRENT_TIMESTAMP + INTERVAL '7 days',
--     CURRENT_TIMESTAMP
-- );

-- =====================================================
-- 5. 权限设置（根据实际环境调整）
-- =====================================================

-- 授予应用用户读写权限
-- GRANT SELECT, INSERT, UPDATE, DELETE ON sessions TO agentflow_app;
-- GRANT SELECT, INSERT, DELETE ON audit_logs TO agentflow_app;

-- =====================================================
-- 迁移完成
-- =====================================================

-- 验证表是否创建成功
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'sessions') THEN
        RAISE NOTICE '✓ sessions 表创建成功';
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'audit_logs') THEN
        RAISE NOTICE '✓ audit_logs 表创建成功';
    END IF;
    
    RAISE NOTICE '✓ Sprint 5 数据库迁移完成';
END $$;
