-- ============================================================
-- 002_rag.sql - RAG 模块（知识库、文档、向量、工具）
-- ============================================================

-- ============================================================
-- 1. 知识库表
-- ============================================================
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL DEFAULT 'document',
    visibility_scope VARCHAR(64) NOT NULL DEFAULT 'tenant',
    default_embedding_model VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    config JSONB DEFAULT '{}'::JSONB,
    metadata JSONB DEFAULT '{}'::JSONB,
    document_count INT DEFAULT 0,
    chunk_count INT DEFAULT 0,
    total_size BIGINT DEFAULT 0,
    created_by VARCHAR(100),
    updated_by VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_kb_tenant_name UNIQUE (tenant_id, name),
    CONSTRAINT check_kb_status CHECK (status IN ('active', 'archived', 'deleted'))
);

CREATE INDEX IF NOT EXISTS idx_kb_tenant ON knowledge_bases(tenant_id);
CREATE INDEX IF NOT EXISTS idx_kb_status ON knowledge_bases(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_kb_deleted_at ON knowledge_bases(deleted_at) WHERE deleted_at IS NULL;

-- ============================================================
-- 2. 文档表
-- ============================================================
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(50) NOT NULL DEFAULT 'text/plain',
    source VARCHAR(500),
    source_type VARCHAR(50),
    source_uri TEXT,
    version VARCHAR(255),
    file_size BIGINT,
    file_mime_type VARCHAR(100),
    file_hash VARCHAR(64),
    char_count INT,
    word_count INT,
    chunk_count INT DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    metadata JSONB DEFAULT '{}'::JSONB,
    uploaded_by VARCHAR(100),
    processed_at TIMESTAMPTZ,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT check_doc_status CHECK (status IN ('pending', 'processing', 'completed', 'indexed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_doc_kb ON documents(knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_doc_tenant ON documents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_doc_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_doc_deleted_at ON documents(deleted_at) WHERE deleted_at IS NULL;

-- ============================================================
-- 3. 文档分块表（支持向量检索）
-- ============================================================
CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    chunk_index INT NOT NULL,
    start_pos INT,
    end_pos INT,
    token_count INT DEFAULT 0,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chunk_doc ON document_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_chunk_kb ON document_chunks(knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_chunk_tenant ON document_chunks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_chunk_embedding ON document_chunks USING hnsw (embedding vector_cosine_ops);

-- ============================================================
-- 4. RAG 查询日志表
-- ============================================================
CREATE TABLE IF NOT EXISTS rag_query_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    knowledge_base_ids UUID[] NOT NULL,
    query TEXT,
    top_k INTEGER NOT NULL DEFAULT 5,
    score_threshold DOUBLE PRECISION,
    retrieved_count INTEGER NOT NULL DEFAULT 0,
    avg_score DOUBLE PRECISION,
    latency_ms INTEGER NOT NULL,
    trace_id VARCHAR(255),
    session_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rag_logs_tenant_created_at ON rag_query_logs(tenant_id, created_at DESC);

-- ============================================================
-- 5. 工具表
-- ============================================================
CREATE TABLE IF NOT EXISTS tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL,
    description TEXT,
    input_schema JSONB NOT NULL DEFAULT '{}'::JSONB,
    output_schema JSONB NOT NULL DEFAULT '{}'::JSONB,
    sensitivity_level VARCHAR(32) NOT NULL DEFAULT 'normal',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_tools_tenant_name UNIQUE (tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_tools_tenant ON tools(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tools_deleted_at ON tools(deleted_at) WHERE deleted_at IS NULL;

-- ============================================================
-- 6. 工具版本表
-- ============================================================
CREATE TABLE IF NOT EXISTS tool_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    version VARCHAR(64) NOT NULL,
    impl_type VARCHAR(32) NOT NULL,
    impl_ref TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tool_versions_tool_version UNIQUE (tool_id, version)
);

-- ============================================================
-- 7. 工具调用日志表
-- ============================================================
CREATE TABLE IF NOT EXISTS tool_call_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    tool_version_id UUID NOT NULL REFERENCES tool_versions(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL,
    latency_ms INTEGER NOT NULL,
    error_code VARCHAR(64),
    error_message TEXT,
    trace_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_call_logs_tenant_created_at ON tool_call_logs(tenant_id, created_at DESC);

-- ============================================================
-- 向量检索辅助函数
-- ============================================================
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
        dc.document_id,
        dc.content,
        1 - (embedding <=> query_embedding) as similarity
    FROM document_chunks dc
    WHERE knowledge_base_id = kb_id AND embedding IS NOT NULL
    ORDER BY embedding <=> query_embedding
    LIMIT match_count;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE VIEW active_knowledge_bases AS SELECT * FROM knowledge_bases WHERE deleted_at IS NULL;
CREATE OR REPLACE VIEW active_documents AS SELECT * FROM documents WHERE deleted_at IS NULL;

COMMENT ON TABLE knowledge_bases IS '知识库表';
COMMENT ON TABLE documents IS '文档表';
COMMENT ON TABLE document_chunks IS '文档分块表（向量检索）';
COMMENT ON TABLE tools IS '工具定义表';
COMMENT ON FUNCTION search_similar_chunks IS '语义检索函数';
