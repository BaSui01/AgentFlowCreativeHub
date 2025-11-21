-- 0009_model_credentials_and_fallback.sql
-- 引入模型凭证存储、模型元数据字段与 Agent 主备模型配置

BEGIN;

-- =====================================================
-- 1. models 表扩展字段
-- =====================================================

ALTER TABLE models
    ADD COLUMN IF NOT EXISTS region VARCHAR(64),
    ADD COLUMN IF NOT EXISTS rate_limit_per_min INTEGER,
    ADD COLUMN IF NOT EXISTS latency_slo_ms INTEGER,
    ADD COLUMN IF NOT EXISTS health_status VARCHAR(32) DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS last_health_check TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS default_credential_id UUID;

CREATE INDEX IF NOT EXISTS idx_models_region ON models(region);
CREATE INDEX IF NOT EXISTS idx_models_health_status ON models(health_status);

-- =====================================================
-- 2. 模型凭证表
-- =====================================================

CREATE TABLE IF NOT EXISTS model_credentials (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    api_key_ciphertext BYTEA NOT NULL,
    base_url TEXT,
    extra_headers JSONB DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_credentials_tenant ON model_credentials(tenant_id);
CREATE INDEX IF NOT EXISTS idx_model_credentials_model ON model_credentials(model_id);

-- =====================================================
-- 3. Agent 主备模型字段
-- =====================================================

ALTER TABLE agent_configs
    RENAME COLUMN IF EXISTS model_id TO primary_model_id;

ALTER TABLE agent_configs
    ADD COLUMN IF NOT EXISTS secondary_model_id UUID,
    ADD COLUMN IF NOT EXISTS fallback_strategy VARCHAR(32) NOT NULL DEFAULT 'auto',
    ADD COLUMN IF NOT EXISTS fallback_timeout_ms INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_fallback_at TIMESTAMPTZ;

UPDATE agent_configs
SET primary_model_id = primary_model_id
WHERE primary_model_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_agent_configs_primary_model ON agent_configs(primary_model_id);
CREATE INDEX IF NOT EXISTS idx_agent_configs_secondary_model ON agent_configs(secondary_model_id);

-- =====================================================
-- 4. 模型调用日志字段
-- =====================================================

ALTER TABLE model_call_logs
    ADD COLUMN IF NOT EXISTS active_model_id UUID,
    ADD COLUMN IF NOT EXISTS fallback_used BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS fallback_model_id UUID,
    ADD COLUMN IF NOT EXISTS error_type VARCHAR(64);

COMMIT;
