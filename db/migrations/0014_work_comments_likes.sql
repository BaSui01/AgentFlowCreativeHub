-- 创建作品评论表
CREATE TABLE IF NOT EXISTS work_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    work_id UUID NOT NULL,
    user_id UUID NOT NULL,
    content TEXT NOT NULL,
    reply_to_id UUID, -- 回复的评论ID（用于嵌套评论）
    like_count INT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    FOREIGN KEY (work_id) REFERENCES published_works(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    FOREIGN KEY (reply_to_id) REFERENCES work_comments(id) ON DELETE CASCADE
);

-- 创建索引
CREATE INDEX idx_work_comments_work_id ON work_comments(work_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_work_comments_user_id ON work_comments(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_work_comments_reply_to_id ON work_comments(reply_to_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_work_comments_tenant_id ON work_comments(tenant_id);
CREATE INDEX idx_work_comments_created_at ON work_comments(created_at DESC);

-- 创建作品点赞表
CREATE TABLE IF NOT EXISTS work_likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    work_id UUID NOT NULL,
    user_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(work_id, user_id), -- 防止重复点赞
    FOREIGN KEY (work_id) REFERENCES works(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 创建索引
CREATE INDEX idx_work_likes_work_id ON work_likes(work_id);
CREATE INDEX idx_work_likes_user_id ON work_likes(user_id);
CREATE INDEX idx_work_likes_tenant_id ON work_likes(tenant_id);

-- 触发器：自动更新 updated_at
CREATE OR REPLACE FUNCTION update_work_comments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_work_comments_updated_at
    BEFORE UPDATE ON work_comments
    FOR EACH ROW
    EXECUTE FUNCTION update_work_comments_updated_at();

-- 触发器：更新作品的评论数
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

CREATE TRIGGER trigger_work_comment_count_insert
    AFTER INSERT ON work_comments
    FOR EACH ROW
    WHEN (NEW.deleted_at IS NULL)
    EXECUTE FUNCTION update_work_comment_count();

CREATE TRIGGER trigger_work_comment_count_delete
    AFTER UPDATE OF deleted_at ON work_comments
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION update_work_comment_count();

-- 触发器：更新作品的点赞数
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

CREATE TRIGGER trigger_work_like_count
    AFTER INSERT OR DELETE ON work_likes
    FOR EACH ROW
    EXECUTE FUNCTION update_work_like_count();
