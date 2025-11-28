package plot

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlotRecommendation 剧情推演记录
type PlotRecommendation struct {
	ID          string         `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID    string         `gorm:"type:varchar(36);not null;index" json:"tenant_id"`
	UserID      string         `gorm:"type:varchar(36);not null;index" json:"user_id"`
	WorkspaceID string         `gorm:"type:varchar(36);index" json:"workspace_id,omitempty"`
	WorkID      string         `gorm:"type:varchar(36);index" json:"work_id,omitempty"`
	ChapterID   string         `gorm:"type:varchar(36);index" json:"chapter_id,omitempty"`
	
	Title           string         `gorm:"type:varchar(200);not null" json:"title"`                       // 推演标题
	CurrentPlot     string         `gorm:"type:text;not null" json:"current_plot"`                       // 当前剧情
	CharacterInfo   string         `gorm:"type:text" json:"character_info,omitempty"`                    // 角色信息
	WorldSetting    string         `gorm:"type:text" json:"world_setting,omitempty"`                     // 世界观设定
	
	Branches        string         `gorm:"type:jsonb;not null" json:"branches"`                          // 生成的分支（JSON数组）
	SelectedBranch  *int           `gorm:"index" json:"selected_branch,omitempty"`                        // 已选择的分支索引
	Applied         bool           `gorm:"default:false;index" json:"applied"`                           // 是否已应用到章节
	AppliedAt       *time.Time     `json:"applied_at,omitempty"`                                         // 应用时间
	
	ModelID         string         `gorm:"type:varchar(36);not null" json:"model_id"`                    // 使用的模型ID
	AgentID         string         `gorm:"type:varchar(36)" json:"agent_id,omitempty"`                   // 使用的Agent ID
	
	Metadata        string         `gorm:"type:jsonb" json:"metadata,omitempty"`                         // 元数据
	
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate GORM Hook
func (p *PlotRecommendation) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// TableName 指定表名
func (PlotRecommendation) TableName() string {
	return "plot_recommendations"
}

// PlotBranch 剧情分支（用于JSON存储）
type PlotBranch struct {
	ID            int      `json:"id"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	KeyEvents     []string `json:"key_events"`
	EmotionalTone string   `json:"emotional_tone"`
	Hook          string   `json:"hook"`
	Difficulty    int      `json:"difficulty"`
}

// CreatePlotRequest 创建剧情推演请求
type CreatePlotRequest struct {
	Title         string         `json:"title" binding:"required,max=200"`
	CurrentPlot   string         `json:"current_plot" binding:"required"`
	CharacterInfo *string        `json:"character_info,omitempty"`
	WorldSetting  *string        `json:"world_setting,omitempty"`
	NumBranches   int            `json:"num_branches" binding:"min=1,max=10"`
	ModelID       string         `json:"model_id" binding:"required"`
	WorkspaceID   *string        `json:"workspace_id,omitempty"`
	WorkID        *string        `json:"work_id,omitempty"`
	ChapterID     *string        `json:"chapter_id,omitempty"`
}

// UpdatePlotRequest 更新剧情推演请求
type UpdatePlotRequest struct {
	Title          *string `json:"title,omitempty"`
	SelectedBranch *int    `json:"selected_branch,omitempty"`
}

// ApplyPlotRequest 应用剧情到章节请求
type ApplyPlotRequest struct {
	PlotID         string `json:"plot_id" binding:"required"`
	ChapterID      string `json:"chapter_id" binding:"required"`
	BranchIndex    int    `json:"branch_index" binding:"min=0"`
	AppendContent  bool   `json:"append_content"`         // 是否追加到现有内容（否则替换）
}

// ListPlotRecommendationsRequest 列表查询请求
type ListPlotRecommendationsRequest struct {
	WorkspaceID *string `form:"workspace_id"`
	WorkID      *string `form:"work_id"`
	ChapterID   *string `form:"chapter_id"`
	Applied     *bool   `form:"applied"`
	Page        int     `form:"page" binding:"min=1"`
	PageSize    int     `form:"page_size" binding:"min=1,max=100"`
}

// PlotRecommendationResponse 剧情推演响应（包含解析后的分支）
type PlotRecommendationResponse struct {
	*PlotRecommendation
	ParsedBranches []PlotBranch `json:"parsed_branches"`
}
