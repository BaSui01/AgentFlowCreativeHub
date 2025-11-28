-- ============================================================
-- 007_content.sql - 内容模块（作品、评论、点赞、片段）
-- ============================================================

-- ============================================================
-- 1. 发布作品表
-- ============================================================
CREATE TABLE IF NOT EXISTS published_works (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID,
    file_id UUID,
    
    -- 作品信息
    title VARCHAR(200) NOT NULL,
    summary VARCHAR(500),
    content TEXT,
    cover_image VARCHAR(500),
    word_count BIGINT DEFAULT 0,
    
    -- 分类
    category_id UUID,
    tags JSONB DEFAULT '[]'::JSONB,
    
    -- 状态
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    
    -- 发布
    published_at TIMESTAMPTZ,
    
    -- 审核
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID,
    reject_reason VARCHAR(500),
    
    -- 统计
    view_count BIGINT DEFAULT 0,
    like_count BIGINT DEFAULT 0,
    comment_count BIGINT DEFAULT 0,
    share_count BIGINT DEFAULT 0,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_published_works_tenant ON published_works(tenant_id);
CREATE INDEX IF NOT EXISTS idx_published_works_user ON published_works(user_id);
CREATE INDEX IF NOT EXISTS idx_published_works_status ON published_works(status);
CREATE INDEX IF NOT EXISTS idx_published_works_published ON published_works(published_at DESC);

-- ============================================================
-- 2. 作品评论表
-- ============================================================
CREATE TABLE IF NOT EXISTS work_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    work_id UUID NOT NULL REFERENCES published_works(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 评论内容
    content TEXT NOT NULL,
    
    -- 回复
    reply_to_id UUID REFERENCES work_comments(id) ON DELETE CASCADE,
    
    -- 统计
    like_count INT DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_work_comments_work_id ON work_comments(work_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_comments_user_id ON work_comments(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_comments_reply_to_id ON work_comments(reply_to_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_work_comments_tenant_id ON work_comments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_work_comments_created_at ON work_comments(created_at DESC);

-- ============================================================
-- 3. 作品点赞表
-- ============================================================
CREATE TABLE IF NOT EXISTS work_likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    work_id UUID NOT NULL REFERENCES published_works(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(work_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_work_likes_work_id ON work_likes(work_id);
CREATE INDEX IF NOT EXISTS idx_work_likes_user_id ON work_likes(user_id);
CREATE INDEX IF NOT EXISTS idx_work_likes_tenant_id ON work_likes(tenant_id);

-- ============================================================
-- 4. 片段管理表
-- ============================================================
CREATE TABLE IF NOT EXISTS fragments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID,
    work_id UUID,
    chapter_id UUID,
    
    -- 片段信息
    type VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,
    
    -- 优先级
    priority INT DEFAULT 0,
    due_date TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    
    -- 排序
    sort_order INT DEFAULT 0,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_fragments_tenant ON fragments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_fragments_user ON fragments(user_id);
CREATE INDEX IF NOT EXISTS idx_fragments_workspace ON fragments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_fragments_work ON fragments(work_id);
CREATE INDEX IF NOT EXISTS idx_fragments_type ON fragments(type);
CREATE INDEX IF NOT EXISTS idx_fragments_status ON fragments(status);
CREATE INDEX IF NOT EXISTS idx_fragments_deleted_at ON fragments(deleted_at);

-- ============================================================
-- 5. 多模型抽卡历史表
-- ============================================================
CREATE TABLE IF NOT EXISTS draw_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 抽卡信息
    agent_type VARCHAR(50) NOT NULL,
    model_ids TEXT NOT NULL,
    input_prompt TEXT NOT NULL,
    
    -- 结果
    results JSONB NOT NULL,
    best_model_id UUID,
    
    -- 性能
    total_time_ms BIGINT,
    success_rate FLOAT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_draw_histories_tenant ON draw_histories(tenant_id);
CREATE INDEX IF NOT EXISTS idx_draw_histories_user ON draw_histories(user_id);
CREATE INDEX IF NOT EXISTS idx_draw_histories_agent_type ON draw_histories(agent_type);
CREATE INDEX IF NOT EXISTS idx_draw_histories_created_at ON draw_histories(created_at);

-- ============================================================
-- 6. 剧情推演表
-- ============================================================
CREATE TABLE IF NOT EXISTS plot_recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID,
    work_id UUID,
    chapter_id UUID,
    
    -- 推演信息
    title VARCHAR(200) NOT NULL,
    current_plot TEXT NOT NULL,
    character_info TEXT,
    world_setting TEXT,
    
    -- 分支
    branches JSONB NOT NULL,
    selected_branch INT,
    
    -- 应用
    applied BOOLEAN DEFAULT FALSE,
    applied_at TIMESTAMPTZ,
    
    -- 来源
    model_id UUID NOT NULL,
    agent_id UUID,
    
    -- 元数据
    metadata JSONB DEFAULT '{}'::JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_plot_recommendations_tenant ON plot_recommendations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plot_recommendations_user ON plot_recommendations(user_id);
CREATE INDEX IF NOT EXISTS idx_plot_recommendations_workspace ON plot_recommendations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_plot_recommendations_work ON plot_recommendations(work_id);
CREATE INDEX IF NOT EXISTS idx_plot_recommendations_applied ON plot_recommendations(applied);
CREATE INDEX IF NOT EXISTS idx_plot_recommendations_deleted_at ON plot_recommendations(deleted_at);

COMMENT ON TABLE published_works IS '发布作品表';
COMMENT ON TABLE work_comments IS '作品评论表';
COMMENT ON TABLE work_likes IS '作品点赞表';
COMMENT ON TABLE fragments IS '片段管理表（灵感、素材、待办、笔记）';
COMMENT ON TABLE draw_histories IS '多模型抽卡历史表';
COMMENT ON TABLE plot_recommendations IS '剧情推演表';
