-- ============================================================
-- 003_models.sql - AI 模型模块
-- ============================================================

-- ============================================================
-- 1. 模型配置表
-- ============================================================
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    model_identifier VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    type VARCHAR(50) DEFAULT 'chat',
    category VARCHAR(50) DEFAULT 'chat',
    description TEXT,
    
    -- 定价
    input_cost_per_1k DECIMAL(10, 6) DEFAULT 0,
    output_cost_per_1k DECIMAL(10, 6) DEFAULT 0,
    
    -- 能力限制
    context_window INT DEFAULT 4096,
    max_output_tokens INT DEFAULT 4096,
    
    -- 特性
    features JSONB DEFAULT '{}'::JSONB,
    supported_languages TEXT[] DEFAULT ARRAY['zh', 'en'],
    
    -- API 配置
    base_url VARCHAR(500),
    api_version VARCHAR(50),
    api_format VARCHAR(50) DEFAULT 'openai',
    region VARCHAR(64),
    
    -- 健康状态
    health_status VARCHAR(32) DEFAULT 'unknown',
    last_health_check TIMESTAMPTZ,
    rate_limit_per_min INTEGER,
    latency_slo_ms INTEGER,
    
    -- 状态
    status VARCHAR(32) DEFAULT 'active',
    is_builtin BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    
    -- 默认凭证
    default_credential_id UUID,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    last_synced_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_models_tenant ON models(tenant_id);
CREATE INDEX IF NOT EXISTS idx_models_provider ON models(provider);
CREATE INDEX IF NOT EXISTS idx_models_provider_category ON models(provider, category) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_models_is_active ON models(is_active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_models_health_status ON models(health_status);

-- ============================================================
-- 2. 模型凭证表
-- ============================================================
CREATE TABLE IF NOT EXISTS model_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    api_key_ciphertext BYTEA NOT NULL,
    base_url TEXT,
    extra_headers JSONB DEFAULT '{}'::JSONB,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_credentials_tenant ON model_credentials(tenant_id);
CREATE INDEX IF NOT EXISTS idx_model_credentials_model ON model_credentials(model_id);

-- ============================================================
-- 3. 模型调用日志表
-- ============================================================
CREATE TABLE IF NOT EXISTS model_call_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    active_model_id UUID,
    
    -- 调用信息
    request_id VARCHAR(255),
    agent_id UUID,
    workflow_id UUID,
    
    -- Token 统计
    prompt_tokens INT DEFAULT 0,
    completion_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    
    -- 成本
    cost_usd DECIMAL(10, 6) DEFAULT 0,
    
    -- 性能
    latency_ms INT,
    
    -- 状态
    status VARCHAR(32) NOT NULL DEFAULT 'success',
    error_code VARCHAR(64),
    error_message TEXT,
    error_type VARCHAR(64),
    
    -- 回退
    fallback_used BOOLEAN DEFAULT FALSE,
    fallback_model_id UUID,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_call_logs_tenant ON model_call_logs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_model_call_logs_model ON model_call_logs(model_id);
CREATE INDEX IF NOT EXISTS idx_model_call_logs_user ON model_call_logs(user_id);

-- ============================================================
-- 4. 模型配额表
-- ============================================================
CREATE TABLE IF NOT EXISTS model_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    model_id UUID REFERENCES models(id) ON DELETE CASCADE,
    
    -- 限制
    max_tokens_per_day BIGINT DEFAULT 0,
    max_tokens_per_month BIGINT DEFAULT 0,
    max_requests_per_minute INT DEFAULT 0,
    max_cost_per_day DECIMAL(10, 2) DEFAULT 0,
    max_cost_per_month DECIMAL(10, 2) DEFAULT 0,
    
    -- 使用量
    used_tokens_today BIGINT DEFAULT 0,
    used_tokens_month BIGINT DEFAULT 0,
    used_cost_today DECIMAL(10, 2) DEFAULT 0,
    used_cost_month DECIMAL(10, 2) DEFAULT 0,
    
    -- 重置时间
    last_reset_daily TIMESTAMPTZ,
    last_reset_monthly TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_quotas_tenant ON model_quotas(tenant_id);
CREATE INDEX IF NOT EXISTS idx_model_quotas_model ON model_quotas(model_id);

COMMENT ON TABLE models IS 'AI 模型配置表';
COMMENT ON TABLE model_credentials IS '模型凭证表（API Key 加密存储）';
COMMENT ON TABLE model_call_logs IS '模型调用日志表';
COMMENT ON TABLE model_quotas IS '模型配额表';

-- ============================================================
-- 5. 添加会员等级权限控制字段
-- ============================================================
ALTER TABLE models ADD COLUMN IF NOT EXISTS allowed_tiers JSONB DEFAULT '[]'::JSONB;
COMMENT ON COLUMN models.allowed_tiers IS '允许访问的会员等级，如 ["free", "basic", "pro", "enterprise"]，空数组表示所有等级可用';
CREATE INDEX IF NOT EXISTS idx_models_allowed_tiers ON models USING GIN(allowed_tiers);
