-- Migration: 为所有表添加软删除和补充字段
-- Description: 添加 deleted_at, deleted_by 字段支持软删除；补充用户、租户、审计日志等表的缺失字段
-- Date: 2025-11-17

-- ==========================================
-- 1. 为 tenants 表添加软删除和补充字段
-- ==========================================

ALTER TABLE tenants 
    ADD COLUMN IF NOT EXISTS tier VARCHAR(50) NOT NULL DEFAULT 'free',
    ADD COLUMN IF NOT EXISTS contact_email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS contact_phone VARCHAR(50),
    ADD COLUMN IF NOT EXISTS contact_person VARCHAR(100),
    ADD COLUMN IF NOT EXISTS industry VARCHAR(100),
    ADD COLUMN IF NOT EXISTS company_size VARCHAR(50),
    ADD COLUMN IF NOT EXISTS country VARCHAR(100),
    ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS subscription_ends_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(100);

-- 添加约束
ALTER TABLE tenants 
    ADD CONSTRAINT IF NOT EXISTS check_tenants_tier 
    CHECK (tier IN ('free', 'pro', 'enterprise'));

-- 添加索引（部分索引，仅索引未删除记录）
CREATE INDEX IF NOT EXISTS idx_tenants_deleted_at 
    ON tenants(deleted_at) 
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tenants_tier 
    ON tenants(tier);

COMMENT ON COLUMN tenants.tier IS '套餐层级: free, pro, enterprise';
COMMENT ON COLUMN tenants.deleted_at IS '软删除时间，NULL 表示未删除';

-- ==========================================
-- 2. 为 users 表添加软删除和补充字段
-- ==========================================

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS full_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS avatar_url TEXT,
    ADD COLUMN IF NOT EXISTS phone VARCHAR(50),
    ADD COLUMN IF NOT EXISTS locale VARCHAR(10) DEFAULT 'zh-CN',
    ADD COLUMN IF NOT EXISTS timezone VARCHAR(50) DEFAULT 'Asia/Shanghai',
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(50),
    ADD COLUMN IF NOT EXISTS login_attempts INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS deleted_by VARCHAR(100);

-- 添加约束
ALTER TABLE users 
    ADD CONSTRAINT IF NOT EXISTS check_users_status 
    CHECK (status IN ('active', 'inactive', 'locked', 'pending_activation'));

ALTER TABLE users 
    ADD CONSTRAINT IF NOT EXISTS check_users_login_attempts 
    CHECK (login_attempts >= 0 AND login_attempts <= 10);

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_users_deleted_at 
    ON users(deleted_at) 
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_users_email_verified 
    ON users(tenant_id, email_verified);

CREATE INDEX IF NOT EXISTS idx_users_last_login_at 
    ON users(last_login_at DESC);

CREATE INDEX IF NOT EXISTS idx_users_phone 
    ON users(phone) 
    WHERE phone IS NOT NULL;

COMMENT ON COLUMN users.email_verified IS '邮箱验证状态，影响登录权限';
COMMENT ON COLUMN users.login_attempts IS '连续登录失败次数，达到5次将锁定账户';
COMMENT ON COLUMN users.locked_until IS '账户锁定截止时间';

-- ==========================================
-- 3. 为 roles 表添加软删除和补充字段
-- ==========================================

ALTER TABLE roles
    ADD COLUMN IF NOT EXISTS code VARCHAR(100),
    ADD COLUMN IF NOT EXISTS is_system BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS priority INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 添加约束
ALTER TABLE roles 
    ADD CONSTRAINT IF NOT EXISTS check_roles_priority 
    CHECK (priority >= 0 AND priority <= 100);

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at 
    ON roles(deleted_at) 
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_tenant_code 
    ON roles(tenant_id, code) 
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_roles_is_system 
    ON roles(is_system);

COMMENT ON COLUMN roles.code IS '角色代码，如 admin, editor, viewer';
COMMENT ON COLUMN roles.is_system IS '系统预置角色，不可删除';
COMMENT ON COLUMN roles.is_default IS '默认角色，新用户自动分配';

-- ==========================================
-- 4. 为 knowledge_bases 表添加软删除和补充字段
-- ==========================================

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(100),
    ADD COLUMN IF NOT EXISTS updated_by VARCHAR(100),
    ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS document_count INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS chunk_count INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_size BIGINT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 添加约束
ALTER TABLE knowledge_bases 
    ADD CONSTRAINT IF NOT EXISTS check_kb_status 
    CHECK (status IN ('active', 'archived', 'deleted'));

ALTER TABLE knowledge_bases 
    ADD CONSTRAINT IF NOT EXISTS check_kb_counts 
    CHECK (document_count >= 0 AND chunk_count >= 0 AND total_size >= 0);

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_kb_deleted_at 
    ON knowledge_bases(deleted_at) 
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_kb_status 
    ON knowledge_bases(tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_kb_created_by 
    ON knowledge_bases(created_by);

COMMENT ON COLUMN knowledge_bases.document_count IS '文档数量统计';
COMMENT ON COLUMN knowledge_bases.total_size IS '总大小（字节）';

-- ==========================================
-- 5. 为 knowledge_documents 表添加软删除和补充字段
-- ==========================================

ALTER TABLE knowledge_documents
    ADD COLUMN IF NOT EXISTS title VARCHAR(500),
    ADD COLUMN IF NOT EXISTS author VARCHAR(255),
    ADD COLUMN IF NOT EXISTS file_size BIGINT,
    ADD COLUMN IF NOT EXISTS file_mime_type VARCHAR(100),
    ADD COLUMN IF NOT EXISTS file_hash VARCHAR(64),
    ADD COLUMN IF NOT EXISTS uploaded_by VARCHAR(100),
    ADD COLUMN IF NOT EXISTS processed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS error_message TEXT,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 添加约束
ALTER TABLE knowledge_documents 
    ADD CONSTRAINT IF NOT EXISTS check_kd_status 
    CHECK (status IN ('pending', 'processing', 'indexed', 'failed', 'pending_index'));

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_kd_deleted_at 
    ON knowledge_documents(deleted_at) 
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_kd_file_hash 
    ON knowledge_documents(file_hash) 
    WHERE file_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_kd_uploaded_by 
    ON knowledge_documents(uploaded_by);

CREATE INDEX IF NOT EXISTS idx_kd_status 
    ON knowledge_documents(knowledge_base_id, status);

COMMENT ON COLUMN knowledge_documents.file_hash IS 'SHA-256 文件哈希，用于去重';
COMMENT ON COLUMN knowledge_documents.processed_at IS '文档处理完成时间';

-- ==========================================
-- 6. 为 knowledge_chunks 表添加补充字段
-- ==========================================

ALTER TABLE knowledge_chunks
    ADD COLUMN IF NOT EXISTS token_count INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- 添加约束
ALTER TABLE knowledge_chunks 
    ADD CONSTRAINT IF NOT EXISTS check_kc_token_count 
    CHECK (token_count >= 0);

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_kc_created_at 
    ON knowledge_chunks(created_at DESC);

COMMENT ON COLUMN knowledge_chunks.token_count IS 'Token 数量，用于成本计算';

-- ==========================================
-- 7. 为 audit_logs 表添加补充字段
-- ==========================================

ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS resource_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS old_value JSONB,
    ADD COLUMN IF NOT EXISTS new_value JSONB,
    ADD COLUMN IF NOT EXISTS ip_address VARCHAR(50),
    ADD COLUMN IF NOT EXISTS user_agent VARCHAR(500),
    ADD COLUMN IF NOT EXISTS request_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS session_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'success',
    ADD COLUMN IF NOT EXISTS error_message TEXT;

-- 添加约束
ALTER TABLE audit_logs 
    ADD CONSTRAINT IF NOT EXISTS check_audit_status 
    CHECK (status IN ('success', 'failure', 'error'));

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id 
    ON audit_logs(request_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_session_id 
    ON audit_logs(session_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_status 
    ON audit_logs(tenant_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_id 
    ON audit_logs(resource_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_ip_address 
    ON audit_logs(ip_address);

COMMENT ON COLUMN audit_logs.request_id IS '请求 ID，用于追踪单个请求的所有日志';
COMMENT ON COLUMN audit_logs.old_value IS '修改前的值（JSON）';
COMMENT ON COLUMN audit_logs.new_value IS '修改后的值（JSON）';

-- ==========================================
-- 8. 为 tools 表添加软删除
-- ==========================================

ALTER TABLE tools
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_tools_deleted_at 
    ON tools(deleted_at) 
    WHERE deleted_at IS NULL;

-- ==========================================
-- 9. 创建软删除查询视图（性能优化）
-- ==========================================

-- 活跃租户视图
CREATE OR REPLACE VIEW active_tenants AS
SELECT * FROM tenants WHERE deleted_at IS NULL;

-- 活跃用户视图
CREATE OR REPLACE VIEW active_users AS
SELECT * FROM users WHERE deleted_at IS NULL;

-- 活跃角色视图
CREATE OR REPLACE VIEW active_roles AS
SELECT * FROM roles WHERE deleted_at IS NULL;

-- 活跃知识库视图
CREATE OR REPLACE VIEW active_knowledge_bases AS
SELECT * FROM knowledge_bases WHERE deleted_at IS NULL;

-- 活跃文档视图
CREATE OR REPLACE VIEW active_knowledge_documents AS
SELECT * FROM knowledge_documents WHERE deleted_at IS NULL;

-- ==========================================
-- 10. 添加注释说明
-- ==========================================

COMMENT ON TABLE tenants IS '租户表，支持多租户隔离和软删除';
COMMENT ON TABLE users IS '用户表，支持租户级隔离和软删除';
COMMENT ON TABLE roles IS '角色表，支持软删除';
COMMENT ON TABLE knowledge_bases IS '知识库表，支持软删除';
COMMENT ON TABLE knowledge_documents IS '知识文档表，支持软删除';
COMMENT ON TABLE audit_logs IS '审计日志表，记录所有关键操作';

-- 迁移完成
