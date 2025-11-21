-- 增强模型表字段

-- 添加新字段
ALTER TABLE models ADD COLUMN IF NOT EXISTS category VARCHAR(50) DEFAULT 'chat';
ALTER TABLE models ADD COLUMN IF NOT EXISTS features JSONB DEFAULT '{}';
ALTER TABLE models ADD COLUMN IF NOT EXISTS base_url VARCHAR(500);
ALTER TABLE models ADD COLUMN IF NOT EXISTS api_version VARCHAR(50);
ALTER TABLE models ADD COLUMN IF NOT EXISTS api_format VARCHAR(50) DEFAULT 'openai';
ALTER TABLE models ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN DEFAULT false;
ALTER TABLE models ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;
ALTER TABLE models ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

-- 添加注释
COMMENT ON COLUMN models.category IS '模型分类: chat, image, audio, video, embedding, rerank';
COMMENT ON COLUMN models.features IS '模型能力特性: {"vision": true, "function_calling": true, "streaming": true, "cache": true, "json_mode": true}';
COMMENT ON COLUMN models.api_format IS 'API 格式: openai, claude, gemini, deepseek, custom';
COMMENT ON COLUMN models.base_url IS '自定义 API 端点（用于自定义提供商）';
COMMENT ON COLUMN models.api_version IS 'API 版本（主要用于 Azure OpenAI）';
COMMENT ON COLUMN models.is_builtin IS '是否为内置模型（预置模型）';
COMMENT ON COLUMN models.is_active IS '是否启用';
COMMENT ON COLUMN models.last_synced_at IS '最后同步时间（自动发现功能使用）';

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_models_provider_category ON models (provider, category) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_models_is_active ON models (is_active) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_models_api_format ON models (api_format) WHERE deleted_at IS NULL;

-- 更新现有数据
UPDATE models SET category = 'chat' WHERE type = 'chat' AND category IS NULL;
UPDATE models SET category = 'embedding' WHERE type = 'embedding' AND category IS NULL;
UPDATE models SET api_format = 'openai' WHERE provider = 'openai' AND api_format = 'openai';
UPDATE models SET api_format = 'claude' WHERE provider = 'anthropic' AND api_format = 'openai';
UPDATE models SET is_builtin = true WHERE provider IN ('openai', 'anthropic');
UPDATE models SET is_active = true WHERE status = 'active';
