-- 片段管理表
CREATE TABLE IF NOT EXISTS fragments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    workspace_id UUID,
    work_id UUID,
    chapter_id UUID,
    type VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,
    priority INT DEFAULT 0,
    due_date TIMESTAMP,
    completed_at TIMESTAMP,
    metadata JSONB,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_fragments_tenant ON fragments(tenant_id);
CREATE INDEX idx_fragments_user ON fragments(user_id);
CREATE INDEX idx_fragments_workspace ON fragments(workspace_id);
CREATE INDEX idx_fragments_work ON fragments(work_id);
CREATE INDEX idx_fragments_chapter ON fragments(chapter_id);
CREATE INDEX idx_fragments_type ON fragments(type);
CREATE INDEX idx_fragments_status ON fragments(status);
CREATE INDEX idx_fragments_deleted_at ON fragments(deleted_at);

COMMENT ON TABLE fragments IS '片段管理表：灵感片段、素材、待办事项、笔记等';
COMMENT ON COLUMN fragments.type IS '片段类型：inspiration, material, todo, note, reference';
COMMENT ON COLUMN fragments.status IS '状态：pending, completed, archived';
COMMENT ON COLUMN fragments.priority IS '优先级：1-5';

-- 多模型抽卡历史表
CREATE TABLE IF NOT EXISTS draw_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    agent_type VARCHAR(50) NOT NULL,
    model_ids TEXT NOT NULL,
    input_prompt TEXT NOT NULL,
    results JSONB NOT NULL,
    best_model_id UUID,
    total_time_ms BIGINT,
    success_rate FLOAT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_draw_histories_tenant ON draw_histories(tenant_id);
CREATE INDEX idx_draw_histories_user ON draw_histories(user_id);
CREATE INDEX idx_draw_histories_agent_type ON draw_histories(agent_type);
CREATE INDEX idx_draw_histories_created_at ON draw_histories(created_at);

COMMENT ON TABLE draw_histories IS '多模型抽卡历史表：记录多模型并发调用结果';
COMMENT ON COLUMN draw_histories.agent_type IS 'Agent类型：plot, writer等';
COMMENT ON COLUMN draw_histories.results IS 'JSON数组：各模型的生成结果';

-- 剧情推演表
CREATE TABLE IF NOT EXISTS plot_recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    workspace_id UUID,
    work_id UUID,
    chapter_id UUID,
    title VARCHAR(200) NOT NULL,
    current_plot TEXT NOT NULL,
    character_info TEXT,
    world_setting TEXT,
    branches JSONB NOT NULL,
    selected_branch INT,
    applied BOOLEAN DEFAULT FALSE,
    applied_at TIMESTAMP,
    model_id UUID NOT NULL,
    agent_id UUID,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

CREATE INDEX idx_plot_recommendations_tenant ON plot_recommendations(tenant_id);
CREATE INDEX idx_plot_recommendations_user ON plot_recommendations(user_id);
CREATE INDEX idx_plot_recommendations_workspace ON plot_recommendations(workspace_id);
CREATE INDEX idx_plot_recommendations_work ON plot_recommendations(work_id);
CREATE INDEX idx_plot_recommendations_chapter ON plot_recommendations(chapter_id);
CREATE INDEX idx_plot_recommendations_applied ON plot_recommendations(applied);
CREATE INDEX idx_plot_recommendations_deleted_at ON plot_recommendations(deleted_at);

COMMENT ON TABLE plot_recommendations IS '剧情推演表：AI生成的剧情分支推荐';
COMMENT ON COLUMN plot_recommendations.branches IS 'JSON数组：生成的多个剧情分支';
COMMENT ON COLUMN plot_recommendations.selected_branch IS '已选择的分支索引';
COMMENT ON COLUMN plot_recommendations.applied IS '是否已应用到章节';
