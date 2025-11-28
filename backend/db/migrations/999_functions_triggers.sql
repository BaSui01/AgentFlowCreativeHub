-- ============================================================
-- 999_functions_triggers.sql - 通用函数和触发器
-- ============================================================

-- ============================================================
-- 1. 通用 updated_at 触发器函数
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_updated_at_column() IS '自动更新 updated_at 字段为当前时间';

-- ============================================================
-- 2. 为所有表添加 updated_at 触发器
-- ============================================================

-- Core 模块
DROP TRIGGER IF EXISTS update_tenants_updated_at ON tenants;
CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_permissions_updated_at ON permissions;
CREATE TRIGGER update_permissions_updated_at BEFORE UPDATE ON permissions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_sessions_updated_at ON sessions;
CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- RAG 模块
DROP TRIGGER IF EXISTS update_knowledge_bases_updated_at ON knowledge_bases;
CREATE TRIGGER update_knowledge_bases_updated_at BEFORE UPDATE ON knowledge_bases FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_documents_updated_at ON documents;
CREATE TRIGGER update_documents_updated_at BEFORE UPDATE ON documents FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_document_chunks_updated_at ON document_chunks;
CREATE TRIGGER update_document_chunks_updated_at BEFORE UPDATE ON document_chunks FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_tools_updated_at ON tools;
CREATE TRIGGER update_tools_updated_at BEFORE UPDATE ON tools FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Models 模块
DROP TRIGGER IF EXISTS update_models_updated_at ON models;
CREATE TRIGGER update_models_updated_at BEFORE UPDATE ON models FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_model_credentials_updated_at ON model_credentials;
CREATE TRIGGER update_model_credentials_updated_at BEFORE UPDATE ON model_credentials FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Agent 模块
DROP TRIGGER IF EXISTS update_agent_configs_updated_at ON agent_configs;
CREATE TRIGGER update_agent_configs_updated_at BEFORE UPDATE ON agent_configs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_agent_capabilities_updated_at ON agent_capabilities;
CREATE TRIGGER update_agent_capabilities_updated_at BEFORE UPDATE ON agent_capabilities FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Workspace 模块
DROP TRIGGER IF EXISTS update_workspace_nodes_updated_at ON workspace_nodes;
CREATE TRIGGER update_workspace_nodes_updated_at BEFORE UPDATE ON workspace_nodes FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_workspace_files_updated_at ON workspace_files;
CREATE TRIGGER update_workspace_files_updated_at BEFORE UPDATE ON workspace_files FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_workspace_staging_files_updated_at ON workspace_staging_files;
CREATE TRIGGER update_workspace_staging_files_updated_at BEFORE UPDATE ON workspace_staging_files FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Workflow 模块
DROP TRIGGER IF EXISTS update_workflow_templates_updated_at ON workflow_templates;
CREATE TRIGGER update_workflow_templates_updated_at BEFORE UPDATE ON workflow_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_workflows_updated_at ON workflows;
CREATE TRIGGER update_workflows_updated_at BEFORE UPDATE ON workflows FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_workflow_executions_updated_at ON workflow_executions;
CREATE TRIGGER update_workflow_executions_updated_at BEFORE UPDATE ON workflow_executions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Content 模块
DROP TRIGGER IF EXISTS update_published_works_updated_at ON published_works;
CREATE TRIGGER update_published_works_updated_at BEFORE UPDATE ON published_works FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_work_comments_updated_at ON work_comments;
CREATE TRIGGER update_work_comments_updated_at BEFORE UPDATE ON work_comments FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_fragments_updated_at ON fragments;
CREATE TRIGGER update_fragments_updated_at BEFORE UPDATE ON fragments FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 3. 知识库统计触发器
-- ============================================================

-- 文档添加时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_document_insert()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases 
    SET document_count = document_count + 1, updated_at = NOW()
    WHERE id = NEW.knowledge_base_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_doc_insert ON documents;
CREATE TRIGGER trg_update_kb_on_doc_insert AFTER INSERT ON documents FOR EACH ROW EXECUTE FUNCTION update_kb_stats_on_document_insert();

-- 文档删除时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_document_delete()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases 
    SET document_count = GREATEST(0, document_count - 1), updated_at = NOW()
    WHERE id = OLD.knowledge_base_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_doc_delete ON documents;
CREATE TRIGGER trg_update_kb_on_doc_delete AFTER DELETE ON documents FOR EACH ROW EXECUTE FUNCTION update_kb_stats_on_document_delete();

-- 分片添加时更新知识库统计
CREATE OR REPLACE FUNCTION update_kb_stats_on_chunk_insert()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE knowledge_bases kb SET chunk_count = chunk_count + 1, updated_at = NOW()
    FROM documents d WHERE kb.id = d.knowledge_base_id AND d.id = NEW.document_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_update_kb_on_chunk_insert ON document_chunks;
CREATE TRIGGER trg_update_kb_on_chunk_insert AFTER INSERT ON document_chunks FOR EACH ROW EXECUTE FUNCTION update_kb_stats_on_chunk_insert();

-- ============================================================
-- 4. 作品统计触发器
-- ============================================================

-- 评论统计
CREATE OR REPLACE FUNCTION update_work_comment_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE published_works SET comment_count = comment_count + 1 WHERE id = NEW.work_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE published_works SET comment_count = GREATEST(comment_count - 1, 0) WHERE id = OLD.work_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_work_comment_insert ON work_comments;
CREATE TRIGGER trg_work_comment_insert AFTER INSERT ON work_comments FOR EACH ROW WHEN (NEW.deleted_at IS NULL) EXECUTE FUNCTION update_work_comment_count();

DROP TRIGGER IF EXISTS trg_work_comment_delete ON work_comments;
CREATE TRIGGER trg_work_comment_delete AFTER UPDATE OF deleted_at ON work_comments FOR EACH ROW WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL) EXECUTE FUNCTION update_work_comment_count();

-- 点赞统计
CREATE OR REPLACE FUNCTION update_work_like_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE published_works SET like_count = like_count + 1 WHERE id = NEW.work_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE published_works SET like_count = GREATEST(like_count - 1, 0) WHERE id = OLD.work_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_work_like_count ON work_likes;
CREATE TRIGGER trg_work_like_count AFTER INSERT OR DELETE ON work_likes FOR EACH ROW EXECUTE FUNCTION update_work_like_count();

-- ============================================================
-- 5. 清理函数
-- ============================================================

-- 清理过期会话
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE affected_rows INTEGER;
BEGIN
    UPDATE sessions SET is_revoked = TRUE, updated_at = NOW()
    WHERE expires_at < NOW() AND is_revoked = FALSE;
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    RETURN affected_rows;
END;
$$ LANGUAGE plpgsql;

-- 清理旧审计日志（保留最近 N 天）
CREATE OR REPLACE FUNCTION cleanup_old_audit_logs(retention_days INTEGER DEFAULT 90)
RETURNS INTEGER AS $$
DECLARE affected_rows INTEGER;
BEGIN
    DELETE FROM audit_logs WHERE created_at < NOW() - (retention_days || ' days')::INTERVAL;
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    RETURN affected_rows;
END;
$$ LANGUAGE plpgsql;

-- 清理旧通知历史
CREATE OR REPLACE FUNCTION cleanup_old_notification_history(retention_days INTEGER DEFAULT 30)
RETURNS INTEGER AS $$
DECLARE affected_rows INTEGER;
BEGIN
    DELETE FROM notification_history WHERE created_at < NOW() - (retention_days || ' days')::INTERVAL;
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    RETURN affected_rows;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_expired_sessions IS '清理过期会话';
COMMENT ON FUNCTION cleanup_old_audit_logs IS '清理旧审计日志';
COMMENT ON FUNCTION cleanup_old_notification_history IS '清理旧通知历史';

-- ============================================================
-- 完成
-- ============================================================
DO $$
BEGIN
    RAISE NOTICE '✓ 函数和触发器创建完成';
END $$;
