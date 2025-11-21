-- 0007_knowledge_base_rag.sql
-- Sprint 6: RAG 知识库功能数据库迁移
-- 创建日期: 2025-11-17

-- =====================================================
-- 1. 安装 pgvector 扩展
-- =====================================================

-- 创建 pgvector 扩展（用于向量存储和检索）
CREATE EXTENSION IF NOT EXISTS vector;

COMMENT ON EXTENSION vector IS 'pgvector 向量扩展，用于存储和检索向量嵌入';

-- =====================================================
-- 2. 创建 knowledge_bases 表（知识库）
-- =====================================================

CREATE TABLE IF NOT EXISTS knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,          -- document, url, api, database
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, inactive
    config JSONB,                        -- 配置信息（分块大小、重叠等）
    metadata JSONB,                      -- 元数据
    doc_count INT DEFAULT 0,             -- 文档数量
    chunk_count INT DEFAULT 0,           -- 分块数量
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_kb_tenant ON knowledge_bases(tenant_id);
CREATE INDEX idx_kb_status ON knowledge_bases(status);

COMMENT ON TABLE knowledge_bases IS '知识库表';
COMMENT ON COLUMN knowledge_bases.type IS '知识库类型：document（文档）、url（网址）、api（API）、database（数据库）';
COMMENT ON COLUMN knowledge_bases.config IS '配置信息（JSON）：chunk_size、chunk_overlap等';

-- =====================================================
-- 3. 创建 documents 表（文档）
-- =====================================================

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(50) NOT NULL,  -- text/plain, text/markdown, application/pdf
    source VARCHAR(500),                 -- 来源（URL、文件路径）
    source_type VARCHAR(50),             -- file, url, manual
    file_size BIGINT,                    -- 文件大小（字节）
    char_count INT,                      -- 字符数
    word_count INT,                      -- 词数
    chunk_count INT DEFAULT 0,           -- 分块数量
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    error_message TEXT,                  -- 错误信息
    metadata JSONB,                      -- 元数据
    created_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_doc_kb ON documents(knowledge_base_id);
CREATE INDEX idx_doc_tenant ON documents(tenant_id);
CREATE INDEX idx_doc_status ON documents(status);

COMMENT ON TABLE documents IS '文档表';
COMMENT ON COLUMN documents.content_type IS 'MIME 类型：text/plain、text/markdown、application/pdf等';
COMMENT ON COLUMN documents.status IS '状态：pending（待处理）、processing（处理中）、completed（完成）、failed（失败）';

-- =====================================================
-- 4. 创建 document_chunks 表（文档分块）
-- =====================================================

CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    content TEXT NOT NULL,
    chunk_index INT NOT NULL,            -- 分块序号（从 0 开始）
    start_pos INT,                       -- 在原文档中的起始位置
    end_pos INT,                         -- 在原文档中的结束位置
    token_count INT,                     -- Token 数量
    embedding vector(1536),              -- 向量嵌入（OpenAI text-embedding-ada-002: 1536维）
    metadata JSONB,                      -- 元数据
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_chunk_doc ON document_chunks(document_id);
CREATE INDEX idx_chunk_kb ON document_chunks(knowledge_base_id);
CREATE INDEX idx_chunk_tenant ON document_chunks(tenant_id);

-- 创建向量检索索引（使用 HNSW 算法，余弦距离）
CREATE INDEX idx_chunk_embedding ON document_chunks USING hnsw (embedding vector_cosine_ops);

COMMENT ON TABLE document_chunks IS '文档分块表';
COMMENT ON COLUMN document_chunks.embedding IS '向量嵌入（1536维，OpenAI text-embedding-ada-002）';
COMMENT ON INDEX idx_chunk_embedding IS '向量检索索引（HNSW 算法，余弦距离）';

-- =====================================================
-- 5. 创建触发器（自动更新 updated_at）
-- =====================================================

-- 创建更新时间触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为知识库表创建触发器
CREATE TRIGGER update_kb_updated_at
BEFORE UPDATE ON knowledge_bases
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- 为文档表创建触发器
CREATE TRIGGER update_doc_updated_at
BEFORE UPDATE ON documents
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- 为分块表创建触发器
CREATE TRIGGER update_chunk_updated_at
BEFORE UPDATE ON document_chunks
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- 6. 插入示例数据（可选）
-- =====================================================

-- 示例：创建默认知识库
-- INSERT INTO knowledge_bases (tenant_id, name, description, type, config)
-- VALUES (
--     '00000000-0000-0000-0000-000000000001',
--     '默认知识库',
--     '用于存储文档和知识',
--     'document',
--     '{"chunk_size": 500, "chunk_overlap": 50}'::jsonb
-- );

-- =====================================================
-- 7. 向量检索辅助函数
-- =====================================================

-- 语义检索函数（返回最相似的 N 个分块）
CREATE OR REPLACE FUNCTION search_similar_chunks(
    query_embedding vector(1536),
    kb_id UUID,
    match_count INT DEFAULT 5
)
RETURNS TABLE (
    chunk_id UUID,
    document_id UUID,
    content TEXT,
    similarity FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        id as chunk_id,
        document_id,
        content,
        1 - (embedding <=> query_embedding) as similarity
    FROM document_chunks
    WHERE knowledge_base_id = kb_id
      AND embedding IS NOT NULL
    ORDER BY embedding <=> query_embedding
    LIMIT match_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION search_similar_chunks IS '语义检索函数：返回最相似的 N 个文档分块';

-- =====================================================
-- 迁移完成
-- =====================================================

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        RAISE NOTICE '✓ pgvector 扩展安装成功';
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'knowledge_bases') THEN
        RAISE NOTICE '✓ knowledge_bases 表创建成功';
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'documents') THEN
        RAISE NOTICE '✓ documents 表创建成功';
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'document_chunks') THEN
        RAISE NOTICE '✓ document_chunks 表创建成功';
    END IF;
    
    RAISE NOTICE '✓ Sprint 6 数据库迁移完成';
END $$;
