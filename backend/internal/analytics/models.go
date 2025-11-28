package analytics

import "time"

// AuthorDashboard 作者仪表盘数据
type AuthorDashboard struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`

	// 写作统计
	TotalWordCount       int64 `json:"total_word_count"`        // 总字数
	TotalChineseChars    int64 `json:"total_chinese_chars"`     // 中文字符数
	TotalEnglishWords    int64 `json:"total_english_words"`     // 英文单词数
	TotalDocuments       int64 `json:"total_documents"`         // 总文档数
	TotalVersions        int64 `json:"total_versions"`          // 总版本数

	// 写作天数统计
	TotalWritingDays     int   `json:"total_writing_days"`      // 累计写作天数
	ConsecutiveWritingDays int `json:"consecutive_writing_days"` // 连续写作天数
	FirstWritingDate     *time.Time `json:"first_writing_date"`  // 首次写作日期

	// Token 消耗
	TotalTokens          int64   `json:"total_tokens"`           // 总 Token 消耗
	TotalCost            float64 `json:"total_cost"`             // 总成本

	// 时间范围
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// MonthlyReport 月度写作报告
type MonthlyReport struct {
	TenantID string    `json:"tenant_id"`
	UserID   string    `json:"user_id"`
	Year     int       `json:"year"`
	Month    int       `json:"month"`

	// 写作统计
	NewWordCount      int64 `json:"new_word_count"`       // 当月新增字数
	NewDocuments      int64 `json:"new_documents"`        // 当月新增文档
	NewVersions       int64 `json:"new_versions"`         // 当月新增版本
	ActiveWritingDays int   `json:"active_writing_days"`  // 当月活跃写作天数

	// Token 消耗
	TokensUsed       int64   `json:"tokens_used"`          // 当月 Token 消耗
	TokenCost        float64 `json:"token_cost"`           // 当月成本

	// 与上月对比
	WordCountChange   float64 `json:"word_count_change"`   // 字数变化百分比
	TokensUsedChange  float64 `json:"tokens_used_change"`  // Token 变化百分比
}

// TokenTrend Token 消耗趋势数据点
type TokenTrend struct {
	Date       string  `json:"date"`        // 日期 YYYY-MM-DD
	Tokens     int64   `json:"tokens"`      // Token 数
	Cost       float64 `json:"cost"`        // 成本
	CallCount  int64   `json:"call_count"`  // 调用次数
}

// FeatureUsage 功能使用分布
type FeatureUsage struct {
	FeatureName string  `json:"feature_name"` // 功能名称（Agent类型）
	CallCount   int64   `json:"call_count"`   // 调用次数
	TokensUsed  int64   `json:"tokens_used"`  // Token 消耗
	Percentage  float64 `json:"percentage"`   // 占比
}

// ModelPreference 模型偏好
type ModelPreference struct {
	ModelName   string  `json:"model_name"`   // 模型名称
	Provider    string  `json:"provider"`     // 提供商
	CallCount   int64   `json:"call_count"`   // 调用次数
	TokensUsed  int64   `json:"tokens_used"`  // Token 消耗
	Percentage  float64 `json:"percentage"`   // 占比
}

// WritingEfficiency 写作效率分析
type WritingEfficiency struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`

	// 日均产出
	AvgDailyWordCount float64 `json:"avg_daily_word_count"` // 日均字数
	AvgWeeklyWordCount float64 `json:"avg_weekly_word_count"` // 周均字数

	// 每日产出趋势
	DailyOutput []DailyWordCount `json:"daily_output"`

	// 周产出趋势
	WeeklyOutput []WeeklyWordCount `json:"weekly_output"`
}

// DailyWordCount 每日字数统计
type DailyWordCount struct {
	Date      string `json:"date"`       // 日期 YYYY-MM-DD
	WordCount int64  `json:"word_count"` // 字数
	Documents int    `json:"documents"`  // 文档数
}

// WeeklyWordCount 每周字数统计
type WeeklyWordCount struct {
	WeekStart string `json:"week_start"` // 周起始日期
	WeekEnd   string `json:"week_end"`   // 周结束日期
	WordCount int64  `json:"word_count"` // 字数
	Documents int    `json:"documents"`  // 文档数
}

// WritingHabits 写作习惯分析
type WritingHabits struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`

	// 高产时段分析（按小时统计）
	HourlyDistribution []HourlyActivity `json:"hourly_distribution"`

	// 高产时段（前3名）
	PeakHours []int `json:"peak_hours"`

	// 平均写作速度（字/分钟，基于会话时长估算）
	AvgWritingSpeed float64 `json:"avg_writing_speed"`

	// 每周分布
	WeekdayDistribution []WeekdayActivity `json:"weekday_distribution"`
}

// HourlyActivity 每小时活动统计
type HourlyActivity struct {
	Hour       int   `json:"hour"`        // 0-23
	WordCount  int64 `json:"word_count"`  // 字数
	Activities int   `json:"activities"`  // 活动次数
}

// WeekdayActivity 每周几活动统计
type WeekdayActivity struct {
	Weekday    int    `json:"weekday"`     // 0=周日, 1=周一...
	WeekdayName string `json:"weekday_name"` // 周几名称
	WordCount  int64  `json:"word_count"`  // 字数
	Activities int    `json:"activities"`  // 活动次数
}

// RecentActivity 近期活动记录
type RecentActivity struct {
	ID           string    `json:"id"`
	ActivityType string    `json:"activity_type"` // ai_call, file_create, file_update
	Description  string    `json:"description"`
	AgentName    string    `json:"agent_name,omitempty"`
	ModelName    string    `json:"model_name,omitempty"`
	TokensUsed   int       `json:"tokens_used,omitempty"`
	FileName     string    `json:"file_name,omitempty"`
	WordCount    int       `json:"word_count,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AnalyticsQuery 分析查询参数
type AnalyticsQuery struct {
	TenantID  string
	UserID    string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
}
