package bookparser

import (
	"time"

	"gorm.io/datatypes"
)

// AnalysisDimension 分析维度
type AnalysisDimension string

const (
	DimensionStyle     AnalysisDimension = "style"     // 文风叙事
	DimensionPlot      AnalysisDimension = "plot"      // 情节设计
	DimensionCharacter AnalysisDimension = "character" // 人物塑造
	DimensionEmotion   AnalysisDimension = "emotion"   // 读者情绪
	DimensionMeme      AnalysisDimension = "meme"      // 热梗搞笑
	DimensionOutline   AnalysisDimension = "outline"   // 章节大纲
	DimensionAll       AnalysisDimension = "all"       // 全部维度
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// BookParserTask 拆书任务
type BookParserTask struct {
	ID          string         `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	TenantID    string         `json:"tenant_id" gorm:"type:uuid;not null;index"`
	UserID      string         `json:"user_id" gorm:"type:uuid;index"`
	
	// 任务信息
	Title       string         `json:"title" gorm:"size:255;not null"`
	SourceType  string         `json:"source_type" gorm:"size:50;not null"` // upload, url, text
	SourceURL   string         `json:"source_url" gorm:"type:text"`
	Content     string         `json:"content" gorm:"type:text"`
	ContentHash string         `json:"content_hash" gorm:"size:64;index"`
	
	// 分析配置
	Dimensions  datatypes.JSON `json:"dimensions" gorm:"type:jsonb"` // []AnalysisDimension
	ModelID     string         `json:"model_id" gorm:"size:100"`
	
	// 任务状态
	Status      TaskStatus     `json:"status" gorm:"size:20;not null;default:pending;index"`
	Progress    int            `json:"progress" gorm:"default:0"` // 0-100
	ErrorMsg    string         `json:"error_msg" gorm:"type:text"`
	
	// 统计信息
	TotalChunks int            `json:"total_chunks" gorm:"default:0"`
	DoneChunks  int            `json:"done_chunks" gorm:"default:0"`
	TokensUsed  int            `json:"tokens_used" gorm:"default:0"`
	
	// 时间
	StartedAt   *time.Time     `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

func (BookParserTask) TableName() string {
	return "book_parser_tasks"
}

// BookParserResult 拆书分析结果
type BookParserResult struct {
	ID          string            `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	TaskID      string            `json:"task_id" gorm:"type:uuid;not null;index"`
	TenantID    string            `json:"tenant_id" gorm:"type:uuid;not null;index"`
	
	// 分析维度
	Dimension   AnalysisDimension `json:"dimension" gorm:"size:50;not null;index"`
	ChunkIndex  int               `json:"chunk_index" gorm:"default:0"` // 分块索引，0 表示整体分析
	
	// 分析结果
	Analysis    datatypes.JSON    `json:"analysis" gorm:"type:jsonb"`   // 分析内容
	Knowledge   datatypes.JSON    `json:"knowledge" gorm:"type:jsonb"`  // 提取的知识点
	Summary     string            `json:"summary" gorm:"type:text"`     // 摘要
	
	// 元数据
	ModelUsed   string            `json:"model_used" gorm:"size:100"`
	TokensUsed  int               `json:"tokens_used" gorm:"default:0"`
	LatencyMs   int64             `json:"latency_ms" gorm:"default:0"`
	
	CreatedAt   time.Time         `json:"created_at" gorm:"autoCreateTime"`
}

func (BookParserResult) TableName() string {
	return "book_parser_results"
}

// BookKnowledge 提取的创作知识（存入知识库）
type BookKnowledge struct {
	ID            string            `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	TenantID      string            `json:"tenant_id" gorm:"type:uuid;not null;index"`
	TaskID        string            `json:"task_id" gorm:"type:uuid;index"`
	SourceTitle   string            `json:"source_title" gorm:"size:255"`
	
	// 知识分类
	Dimension     AnalysisDimension `json:"dimension" gorm:"size:50;not null;index"`
	Category      string            `json:"category" gorm:"size:100;index"` // 子分类
	Tags          datatypes.JSON    `json:"tags" gorm:"type:jsonb"`         // 标签
	
	// 知识内容
	Title         string            `json:"title" gorm:"size:255;not null"`
	Content       string            `json:"content" gorm:"type:text;not null"`
	Example       string            `json:"example" gorm:"type:text"`       // 示例
	Technique     string            `json:"technique" gorm:"type:text"`     // 可复用技法
	
	// 向量化状态
	IsVectorized  bool              `json:"is_vectorized" gorm:"default:false"`
	VectorID      string            `json:"vector_id" gorm:"size:100"`
	
	// 使用统计
	UseCount      int               `json:"use_count" gorm:"default:0"`
	Rating        float64           `json:"rating" gorm:"type:decimal(3,2);default:0"`
	
	CreatedAt     time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
}

func (BookKnowledge) TableName() string {
	return "book_knowledge"
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	TenantID   string              `json:"-"`
	UserID     string              `json:"-"`
	Title      string              `json:"title" binding:"required"`
	SourceType string              `json:"source_type" binding:"required,oneof=upload url text"`
	SourceURL  string              `json:"source_url"`
	Content    string              `json:"content"`
	Dimensions []AnalysisDimension `json:"dimensions"` // 为空则分析全部维度
	ModelID    string              `json:"model_id"`
}

// TaskListRequest 任务列表请求
type TaskListRequest struct {
	TenantID string     `json:"-"`
	UserID   string     `json:"-"`
	Status   TaskStatus `json:"status"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// SearchKnowledgeRequest 知识搜索请求
type SearchKnowledgeRequest struct {
	TenantID  string            `json:"-"`
	Query     string            `json:"query" binding:"required"`
	Dimension AnalysisDimension `json:"dimension"`
	Category  string            `json:"category"`
	Tags      []string          `json:"tags"`
	Limit     int               `json:"limit"`
}

// TaskProgress 任务进度
type TaskProgress struct {
	TaskID      string     `json:"task_id"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`
	TotalChunks int        `json:"total_chunks"`
	DoneChunks  int        `json:"done_chunks"`
	TokensUsed  int        `json:"tokens_used"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
}

// AnalysisResult 分析结果聚合
type AnalysisResult struct {
	TaskID    string                       `json:"task_id"`
	Title     string                       `json:"title"`
	Status    TaskStatus                   `json:"status"`
	Results   map[AnalysisDimension]any    `json:"results"`
	Knowledge []BookKnowledge              `json:"knowledge"`
	Summary   string                       `json:"summary"`
}

// DimensionAnalysis 单维度分析结果
type DimensionAnalysis struct {
	Dimension  AnalysisDimension `json:"dimension"`
	Findings   []Finding         `json:"findings"`
	Summary    string            `json:"summary"`
	Tips       []string          `json:"actionable_tips"`
}

// Finding 分析发现
type Finding struct {
	Aspect      string `json:"aspect"`
	Observation string `json:"observation"`
	Example     string `json:"example"`
	Technique   string `json:"technique"`
}
