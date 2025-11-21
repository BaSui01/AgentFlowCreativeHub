-- Migration: 添加数据库触发器
-- Description: 自动更新 updated_at 字段；统计字段自动维护
-- Date: 2025-11-17

-- ==========================================
-- 1. 创建通用的 updated_at 触发器函数
-- ==========================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_updated_at_column() 
    IS '自动更新 updated_at 字段为当前时间';

-- ==========================================
-- 2. 为所有表添加 updated_at 触发器
-- ==========================================

-- tenants 表
DROP TRIGGER IF EXISTS update_tenants_updated_at ON tenants;
CREATE TRIGGER update_tenants_updated_at 
    BEFORE UPDATE ON tenants
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- users 表
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- roles 表
DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
CREATE TRIGGER update_roles_updated_at 
    BEFORE UPDATE ON roles
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- knowledge_bases 表
DROP TRIGGER IF EXISTS update_knowledge_bases_updated_at ON knowledge_bases;
CREATE TRIGGER update_knowledge_bases_updated_at 
    BEFORE UPDATE ON knowledge_bases
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- knowledge_documents 表
DROP TRIGGER IF EXISTS update_knowledge_documents_updated_at ON knowledge_documents;
CREATE TRIGGER update_knowledge_documents_updated_at 
    BEFORE UPDATE ON knowledge_documents
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- tools 表
DROP TRIGGER IF EXISTS update_tools_updated_at ON tools;
CREATE TRIGGER update_tools_updated_at 
    BEFORE UPDATE ON tools
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- ==========================================
-- 3. 知识库统计字段自动维护
-- ==========================================

-- 文档添加时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_document_insert()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases 
    SET 
        document_count = document_count + 1,
        updated_at = NOW()
    WHERE id = NEW.knowledge_base_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_doc_insert ON knowledge_documents;
CREATE TRIGGER trg_update_kb_on_doc_insert
    AFTER INSERT ON knowledge_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_kb_stats_on_document_insert();

-- 文档删除时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_document_delete()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases 
    SET 
        document_count = GREATEST(0, document_count - 1),
        updated_at = NOW()
    WHERE id = OLD.knowledge_base_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_doc_delete ON knowledge_documents;
CREATE TRIGGER trg_update_kb_on_doc_delete
    AFTER DELETE ON knowledge_documents
    FOR EACH ROW
    EXECUTE FUNCTION update_kb_stats_on_document_delete();

-- 分片添加时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_chunk_insert()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases kb
    SET 
        chunk_count = chunk_count + 1,
        updated_at = NOW()
    FROM knowledge_documents kd
    WHERE kb.id = kd.knowledge_base_id 
      AND kd.id = NEW.document_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_chunk_insert ON knowledge_chunks;
CREATE TRIGGER trg_update_kb_on_chunk_insert
    AFTER INSERT ON knowledge_chunks
    FOR EACH ROW
    EXECUTE FUNCTION update_kb_stats_on_chunk_insert();

-- 分片删除时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_chunk_delete()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases kb
    SET 
        chunk_count = GREATEST(0, chunk_count - 1),
        updated_at = NOW()
    FROM knowledge_documents kd
    WHERE kb.id = kd.knowledge_base_id 
      AND kd.id = OLD.document_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_chunk_delete ON knowledge_chunks;
CREATE TRIGGER trg_update_kb_on_chunk_delete
    AFTER DELETE ON knowledge_chunks
    FOR EACH ROW
    EXECUTE FUNCTION update_kb_stats_on_chunk_delete();

-- ==========================================
-- 4. 软删除审计触发器（可选）
-- ==========================================

-- 记录软删除操作到审计日志
CREATE OR REPLACE FUNCTION log_soft_delete()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
        -- 记录软删除操作（需要有 audit_logs 表）
        INSERT INTO audit_logs (
            id,
            tenant_id,
            user_id,
            action,
            resource,
            resource_id,
            old_value,
            new_value,
            status,
            created_at
        ) VALUES (
            gen_random_uuid(),
            COALESCE(NEW.tenant_id, '00000000-0000-0000-0000-000000000000'),
            NEW.deleted_by,
            'soft_delete',
            TG_TABLE_NAME,
            NEW.id::TEXT,
            jsonb_build_object('deleted_at', NULL),
            jsonb_build_object('deleted_at', NEW.deleted_at, 'deleted_by', NEW.deleted_by),
            'success',
            NOW()
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为关键表添加软删除审计触发器
DROP TRIGGER IF EXISTS trg_log_tenant_soft_delete ON tenants;
CREATE TRIGGER trg_log_tenant_soft_delete
    AFTER UPDATE ON tenants
    FOR EACH ROW
    WHEN (NEW.deleted_at IS DISTINCT FROM OLD.deleted_at)
    EXECUTE FUNCTION log_soft_delete();

DROP TRIGGER IF EXISTS trg_log_user_soft_delete ON users;
CREATE TRIGGER trg_log_user_soft_delete
    AFTER UPDATE ON users
    FOR EACH ROW
    WHEN (NEW.deleted_at IS DISTINCT FROM OLD.deleted_at)
    EXECUTE FUNCTION log_soft_delete();

DROP TRIGGER IF EXISTS trg_log_kb_soft_delete ON knowledge_bases;
CREATE TRIGGER trg_log_kb_soft_delete
    AFTER UPDATE ON knowledge_bases
    FOR EACH ROW
    WHEN (NEW.deleted_at IS DISTINCT FROM OLD.deleted_at)
    EXECUTE FUNCTION log_soft_delete();

-- ==========================================
-- 5. 添加触发器说明
-- ==========================================

COMMENT ON TRIGGER update_tenants_updated_at ON tenants 
    IS '自动更新 tenants 表的 updated_at 字段';

COMMENT ON TRIGGER trg_update_kb_on_doc_insert ON knowledge_documents 
    IS '文档添加时自动更新知识库的 document_count';

COMMENT ON TRIGGER trg_update_kb_on_chunk_insert ON knowledge_chunks 
    IS '分片添加时自动更新知识库的 chunk_count';

COMMENT ON TRIGGER trg_log_tenant_soft_delete ON tenants 
    IS '租户软删除时自动记录审计日志';

-- 迁移完成
