-- 0008_agent_rag_integration.sql
-- Agent RAG 集成：为 agent_configs 表添加知识库关联字段
-- 创建日期: 2025-11-17

-- =====================================================
-- 1. 为 agent_configs 表添加 RAG 配置字段
-- =====================================================

ALTER TABLE agent_configs
ADD COLUMN IF NOT EXISTS knowledge_base_id UUID,               -- 关联的知识库 ID
ADD COLUMN IF NOT EXISTS rag_enabled BOOLEAN DEFAULT FALSE,    -- 是否启用 RAG
ADD COLUMN IF NOT EXISTS rag_top_k INT DEFAULT 3,              -- RAG 检索数量（TopK）
ADD COLUMN IF NOT EXISTS rag_min_score DECIMAL(3,2) DEFAULT 0.70; -- RAG 最小相似度阈值

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_agent_kb ON agent_configs(knowledge_base_id);

-- 添加外键约束（可选，如果启用外键）
-- ALTER TABLE agent_configs
-- ADD CONSTRAINT fk_agent_knowledge_base
-- FOREIGN KEY (knowledge_base_id) REFERENCES knowledge_bases(id) ON DELETE SET NULL;

-- 添加注释
COMMENT ON COLUMN agent_configs.knowledge_base_id IS '关联的知识库 ID（用于 RAG 增强）';
COMMENT ON COLUMN agent_configs.rag_enabled IS '是否启用 RAG 知识库增强功能';
COMMENT ON COLUMN agent_configs.rag_top_k IS 'RAG 检索返回的文档片段数量（默认 3）';
COMMENT ON COLUMN agent_configs.rag_min_score IS 'RAG 检索的最小相似度阈值（默认 0.7，范围 0-1）';

-- =====================================================
-- 2. 更新现有 Agent 配置（示例）
-- =====================================================

-- 示例：为 writer Agent 启用 RAG（如果有默认知识库）
-- UPDATE agent_configs
-- SET 
--     rag_enabled = TRUE,
--     rag_top_k = 5,
--     rag_min_score = 0.75
-- WHERE agent_type = 'writer'
--   AND knowledge_base_id IS NOT NULL;

-- =====================================================
-- 迁移完成
-- =====================================================

DO $$
BEGIN
    -- 检查字段是否添加成功
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'agent_configs' 
          AND column_name = 'knowledge_base_id'
    ) THEN
        RAISE NOTICE '✓ agent_configs.knowledge_base_id 字段添加成功';
    END IF;
    
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'agent_configs' 
          AND column_name = 'rag_enabled'
    ) THEN
        RAISE NOTICE '✓ agent_configs.rag_enabled 字段添加成功';
    END IF;
    
    RAISE NOTICE '✓ Agent RAG 集成数据库迁移完成';
END $$;
