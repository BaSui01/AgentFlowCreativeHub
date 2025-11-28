package analytics

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"time"
	"unicode"

	"gorm.io/gorm"
)

// Service 写作分析服务
type Service struct {
	db *gorm.DB
}

// NewService 创建分析服务
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GetAuthorDashboard 获取作者仪表盘数据
func (s *Service) GetAuthorDashboard(ctx context.Context, query *AnalyticsQuery) (*AuthorDashboard, error) {
	dashboard := &AuthorDashboard{
		TenantID:    query.TenantID,
		UserID:      query.UserID,
		PeriodStart: time.Now().AddDate(0, 0, -30), // 默认30天
		PeriodEnd:   time.Now(),
	}

	if query.StartTime != nil {
		dashboard.PeriodStart = *query.StartTime
	}
	if query.EndTime != nil {
		dashboard.PeriodEnd = *query.EndTime
	}

	// 统计字数（从文件版本内容）
	wordStats, err := s.calculateTotalWordCount(ctx, query.TenantID, query.UserID)
	if err != nil {
		return nil, fmt.Errorf("计算字数失败: %w", err)
	}
	dashboard.TotalWordCount = wordStats.TotalChars
	dashboard.TotalChineseChars = wordStats.ChineseChars
	dashboard.TotalEnglishWords = wordStats.EnglishWords
	dashboard.TotalDocuments = wordStats.DocumentCount
	dashboard.TotalVersions = wordStats.VersionCount

	// 统计写作天数
	writingDays, err := s.calculateWritingDays(ctx, query.TenantID, query.UserID)
	if err != nil {
		return nil, fmt.Errorf("计算写作天数失败: %w", err)
	}
	dashboard.TotalWritingDays = writingDays.TotalDays
	dashboard.ConsecutiveWritingDays = writingDays.ConsecutiveDays
	dashboard.FirstWritingDate = writingDays.FirstDate

	// 统计 Token 消耗
	tokenStats, err := s.calculateTokenUsage(ctx, query.TenantID, query.UserID, dashboard.PeriodStart, dashboard.PeriodEnd)
	if err != nil {
		return nil, fmt.Errorf("计算Token消耗失败: %w", err)
	}
	dashboard.TotalTokens = tokenStats.TotalTokens
	dashboard.TotalCost = tokenStats.TotalCost

	return dashboard, nil
}

// GetMonthlyReport 获取月度报告
func (s *Service) GetMonthlyReport(ctx context.Context, tenantID, userID string, year, month int) (*MonthlyReport, error) {
	report := &MonthlyReport{
		TenantID: tenantID,
		UserID:   userID,
		Year:     year,
		Month:    month,
	}

	// 当月时间范围
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)

	// 上月时间范围
	startOfLastMonth := startOfMonth.AddDate(0, -1, 0)
	endOfLastMonth := startOfMonth.Add(-time.Second)

	// 当月字数统计
	currentWords, err := s.calculatePeriodWordCount(ctx, tenantID, userID, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	report.NewWordCount = currentWords.TotalChars
	report.NewDocuments = currentWords.DocumentCount
	report.NewVersions = currentWords.VersionCount

	// 当月活跃天数
	activeDays, err := s.countActiveDays(ctx, tenantID, userID, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	report.ActiveWritingDays = activeDays

	// 当月 Token 消耗
	tokenStats, err := s.calculateTokenUsage(ctx, tenantID, userID, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	report.TokensUsed = tokenStats.TotalTokens
	report.TokenCost = tokenStats.TotalCost

	// 上月对比
	lastWords, _ := s.calculatePeriodWordCount(ctx, tenantID, userID, startOfLastMonth, endOfLastMonth)
	lastTokens, _ := s.calculateTokenUsage(ctx, tenantID, userID, startOfLastMonth, endOfLastMonth)

	if lastWords != nil && lastWords.TotalChars > 0 {
		report.WordCountChange = float64(currentWords.TotalChars-lastWords.TotalChars) / float64(lastWords.TotalChars) * 100
	}
	if lastTokens != nil && lastTokens.TotalTokens > 0 {
		report.TokensUsedChange = float64(tokenStats.TotalTokens-lastTokens.TotalTokens) / float64(lastTokens.TotalTokens) * 100
	}

	return report, nil
}

// GetTokenTrend 获取 Token 消耗趋势
func (s *Service) GetTokenTrend(ctx context.Context, tenantID, userID string, days int) ([]TokenTrend, error) {
	if days <= 0 {
		days = 30
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	var results []struct {
		Date      string
		Tokens    int64
		Cost      float64
		CallCount int64
	}

	query := s.db.WithContext(ctx).
		Table("ai_call_logs").
		Select(`
			DATE(created_at) as date,
			SUM(total_tokens) as tokens,
			SUM(cost) as cost,
			COUNT(*) as call_count
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Group("DATE(created_at)").Order("date ASC").Scan(&results).Error; err != nil {
		return nil, err
	}

	trends := make([]TokenTrend, len(results))
	for i, r := range results {
		trends[i] = TokenTrend{
			Date:      r.Date,
			Tokens:    r.Tokens,
			Cost:      r.Cost,
			CallCount: r.CallCount,
		}
	}

	return trends, nil
}

// GetFeatureUsage 获取功能使用分布
func (s *Service) GetFeatureUsage(ctx context.Context, tenantID, userID string, days int) ([]FeatureUsage, error) {
	if days <= 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)

	var results []struct {
		AgentType  string
		CallCount  int64
		TokensUsed int64
	}

	// 从 model_call_logs 按 agent_id 分组统计
	query := s.db.WithContext(ctx).
		Table("model_call_logs").
		Select(`
			COALESCE(agent_id, 'direct') as agent_type,
			COUNT(*) as call_count,
			SUM(total_tokens) as tokens_used
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ?", startDate)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Group("agent_id").Order("tokens_used DESC").Scan(&results).Error; err != nil {
		return nil, err
	}

	// 计算总数用于百分比
	var totalCalls, totalTokens int64
	for _, r := range results {
		totalCalls += r.CallCount
		totalTokens += r.TokensUsed
	}

	usages := make([]FeatureUsage, len(results))
	for i, r := range results {
		percentage := 0.0
		if totalTokens > 0 {
			percentage = float64(r.TokensUsed) / float64(totalTokens) * 100
		}
		usages[i] = FeatureUsage{
			FeatureName: r.AgentType,
			CallCount:   r.CallCount,
			TokensUsed:  r.TokensUsed,
			Percentage:  percentage,
		}
	}

	return usages, nil
}

// GetModelPreference 获取模型偏好分析
func (s *Service) GetModelPreference(ctx context.Context, tenantID, userID string, days int) ([]ModelPreference, error) {
	if days <= 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)

	var results []struct {
		ModelName  string
		Provider   string
		CallCount  int64
		TokensUsed int64
	}

	query := s.db.WithContext(ctx).
		Table("ai_call_logs").
		Select(`
			model_name,
			model_provider as provider,
			COUNT(*) as call_count,
			SUM(total_tokens) as tokens_used
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ?", startDate)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Group("model_name, model_provider").Order("tokens_used DESC").Scan(&results).Error; err != nil {
		return nil, err
	}

	// 计算百分比
	var totalTokens int64
	for _, r := range results {
		totalTokens += r.TokensUsed
	}

	prefs := make([]ModelPreference, len(results))
	for i, r := range results {
		percentage := 0.0
		if totalTokens > 0 {
			percentage = float64(r.TokensUsed) / float64(totalTokens) * 100
		}
		prefs[i] = ModelPreference{
			ModelName:  r.ModelName,
			Provider:   r.Provider,
			CallCount:  r.CallCount,
			TokensUsed: r.TokensUsed,
			Percentage: percentage,
		}
	}

	return prefs, nil
}

// GetWritingEfficiency 获取写作效率分析
func (s *Service) GetWritingEfficiency(ctx context.Context, tenantID, userID string, days int) (*WritingEfficiency, error) {
	if days <= 0 {
		days = 30
	}

	efficiency := &WritingEfficiency{
		TenantID: tenantID,
		UserID:   userID,
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	// 每日产出统计
	var dailyResults []struct {
		Date      string
		WordCount int64
		Documents int
	}

	if err := s.db.WithContext(ctx).
		Table("workspace_file_versions v").
		Select(`
			DATE(v.created_at) as date,
			COUNT(DISTINCT v.file_id) as documents,
			0 as word_count
		`).
		Joins("JOIN workspace_files f ON f.id = v.file_id").
		Where("v.tenant_id = ?", tenantID).
		Where("v.created_at BETWEEN ? AND ?", startDate, endDate).
		Group("DATE(v.created_at)").
		Order("date ASC").
		Scan(&dailyResults).Error; err != nil {
		return nil, err
	}

	// 计算每日字数（需要查询内容）
	efficiency.DailyOutput = make([]DailyWordCount, len(dailyResults))
	var totalWords int64
	for i, r := range dailyResults {
		// 计算该天的字数
		dayStart, _ := time.Parse("2006-01-02", r.Date)
		dayEnd := dayStart.Add(24 * time.Hour)
		dayWords, _ := s.calculatePeriodWordCount(ctx, tenantID, userID, dayStart, dayEnd)

		wordCount := int64(0)
		if dayWords != nil {
			wordCount = dayWords.TotalChars
		}

		efficiency.DailyOutput[i] = DailyWordCount{
			Date:      r.Date,
			WordCount: wordCount,
			Documents: r.Documents,
		}
		totalWords += wordCount
	}

	// 计算平均值
	if len(dailyResults) > 0 {
		efficiency.AvgDailyWordCount = float64(totalWords) / float64(len(dailyResults))
	}

	// 周产出统计（按周聚合）
	efficiency.WeeklyOutput = s.aggregateWeeklyOutput(efficiency.DailyOutput)
	if len(efficiency.WeeklyOutput) > 0 {
		var weeklyTotal int64
		for _, w := range efficiency.WeeklyOutput {
			weeklyTotal += w.WordCount
		}
		efficiency.AvgWeeklyWordCount = float64(weeklyTotal) / float64(len(efficiency.WeeklyOutput))
	}

	return efficiency, nil
}

// GetWritingHabits 获取写作习惯分析
func (s *Service) GetWritingHabits(ctx context.Context, tenantID, userID string, days int) (*WritingHabits, error) {
	if days <= 0 {
		days = 30
	}

	habits := &WritingHabits{
		TenantID:           tenantID,
		UserID:             userID,
		HourlyDistribution: make([]HourlyActivity, 24),
		WeekdayDistribution: make([]WeekdayActivity, 7),
	}

	// 初始化
	weekdayNames := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	for i := 0; i < 24; i++ {
		habits.HourlyDistribution[i] = HourlyActivity{Hour: i}
	}
	for i := 0; i < 7; i++ {
		habits.WeekdayDistribution[i] = WeekdayActivity{Weekday: i, WeekdayName: weekdayNames[i]}
	}

	startDate := time.Now().AddDate(0, 0, -days)

	// 查询文件版本创建时间分布
	var results []struct {
		Hour       int
		Weekday    int
		Activities int
	}

	if err := s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Select(`
			EXTRACT(HOUR FROM created_at) as hour,
			EXTRACT(DOW FROM created_at) as weekday,
			COUNT(*) as activities
		`).
		Where("tenant_id = ?", tenantID).
		Where("created_at >= ?", startDate).
		Group("EXTRACT(HOUR FROM created_at), EXTRACT(DOW FROM created_at)").
		Scan(&results).Error; err != nil {
		return nil, err
	}

	// 填充数据
	for _, r := range results {
		if r.Hour >= 0 && r.Hour < 24 {
			habits.HourlyDistribution[r.Hour].Activities += r.Activities
		}
		if r.Weekday >= 0 && r.Weekday < 7 {
			habits.WeekdayDistribution[r.Weekday].Activities += r.Activities
		}
	}

	// 找出高产时段（前3名）
	type hourCount struct {
		hour  int
		count int
	}
	hourCounts := make([]hourCount, 24)
	for i, h := range habits.HourlyDistribution {
		hourCounts[i] = hourCount{hour: i, count: h.Activities}
	}
	sort.Slice(hourCounts, func(i, j int) bool {
		return hourCounts[i].count > hourCounts[j].count
	})
	habits.PeakHours = make([]int, 0, 3)
	for i := 0; i < 3 && i < len(hourCounts); i++ {
		if hourCounts[i].count > 0 {
			habits.PeakHours = append(habits.PeakHours, hourCounts[i].hour)
		}
	}

	return habits, nil
}

// GetRecentActivities 获取近期活动记录
func (s *Service) GetRecentActivities(ctx context.Context, tenantID, userID string, limit int) ([]RecentActivity, error) {
	if limit <= 0 {
		limit = 20
	}

	activities := make([]RecentActivity, 0)

	// AI 调用记录
	var aiLogs []struct {
		ID         string
		ModelName  string
		TotalTokens int
		CreatedAt  time.Time
	}

	aiQuery := s.db.WithContext(ctx).
		Table("ai_call_logs").
		Select("id, model_name, total_tokens, created_at").
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit)

	if userID != "" {
		aiQuery = aiQuery.Where("user_id = ?", userID)
	}

	if err := aiQuery.Scan(&aiLogs).Error; err == nil {
		for _, log := range aiLogs {
			activities = append(activities, RecentActivity{
				ID:           log.ID,
				ActivityType: "ai_call",
				Description:  fmt.Sprintf("调用 %s", log.ModelName),
				ModelName:    log.ModelName,
				TokensUsed:   log.TotalTokens,
				CreatedAt:    log.CreatedAt,
			})
		}
	}

	// 文件创建/更新记录
	var fileLogs []struct {
		ID        string
		FileName  string
		CreatedAt time.Time
	}

	fileQuery := s.db.WithContext(ctx).
		Table("workspace_file_versions v").
		Select("v.id, n.name as file_name, v.created_at").
		Joins("JOIN workspace_files f ON f.id = v.file_id").
		Joins("JOIN workspace_nodes n ON n.id = f.node_id").
		Where("v.tenant_id = ?", tenantID).
		Order("v.created_at DESC").
		Limit(limit)

	if err := fileQuery.Scan(&fileLogs).Error; err == nil {
		for _, log := range fileLogs {
			activities = append(activities, RecentActivity{
				ID:           log.ID,
				ActivityType: "file_update",
				Description:  fmt.Sprintf("更新文件 %s", log.FileName),
				FileName:     log.FileName,
				CreatedAt:    log.CreatedAt,
			})
		}
	}

	// 按时间排序并截取
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].CreatedAt.After(activities[j].CreatedAt)
	})

	if len(activities) > limit {
		activities = activities[:limit]
	}

	return activities, nil
}

// ============================================================================
// 内部辅助方法
// ============================================================================

type wordStats struct {
	TotalChars    int64
	ChineseChars  int64
	EnglishWords  int64
	DocumentCount int64
	VersionCount  int64
}

type writingDaysStats struct {
	TotalDays      int
	ConsecutiveDays int
	FirstDate      *time.Time
}

type tokenStats struct {
	TotalTokens int64
	TotalCost   float64
}

// calculateTotalWordCount 计算总字数
func (s *Service) calculateTotalWordCount(ctx context.Context, tenantID, userID string) (*wordStats, error) {
	stats := &wordStats{}

	// 获取所有最新版本的内容
	var contents []string

	query := s.db.WithContext(ctx).
		Table("workspace_file_versions v").
		Select("v.content").
		Joins("JOIN workspace_files f ON f.id = v.file_id AND f.latest_version_id = v.id").
		Where("v.tenant_id = ?", tenantID)

	if err := query.Pluck("content", &contents).Error; err != nil {
		return nil, err
	}

	// 统计字数
	for _, content := range contents {
		chinese, english := countWords(content)
		stats.ChineseChars += int64(chinese)
		stats.EnglishWords += int64(english)
		stats.TotalChars += int64(chinese + english)
	}

	// 统计文档数和版本数
	s.db.WithContext(ctx).
		Table("workspace_files").
		Where("tenant_id = ?", tenantID).
		Count(&stats.DocumentCount)

	s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Where("tenant_id = ?", tenantID).
		Count(&stats.VersionCount)

	return stats, nil
}

// calculatePeriodWordCount 计算时间段内字数
func (s *Service) calculatePeriodWordCount(ctx context.Context, tenantID, userID string, start, end time.Time) (*wordStats, error) {
	stats := &wordStats{}

	var contents []string
	query := s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Select("content").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end)

	if err := query.Pluck("content", &contents).Error; err != nil {
		return nil, err
	}

	for _, content := range contents {
		chinese, english := countWords(content)
		stats.ChineseChars += int64(chinese)
		stats.EnglishWords += int64(english)
		stats.TotalChars += int64(chinese + english)
	}

	// 统计版本数
	s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Count(&stats.VersionCount)

	// 统计文档数（去重）
	s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Select("COUNT(DISTINCT file_id)").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Scan(&stats.DocumentCount)

	return stats, nil
}

// calculateWritingDays 计算写作天数
func (s *Service) calculateWritingDays(ctx context.Context, tenantID, userID string) (*writingDaysStats, error) {
	stats := &writingDaysStats{}

	// 获取所有写作日期
	var dates []time.Time
	if err := s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Select("DISTINCT DATE(created_at)").
		Where("tenant_id = ?", tenantID).
		Order("DATE(created_at) ASC").
		Pluck("DATE(created_at)", &dates).Error; err != nil {
		return nil, err
	}

	if len(dates) == 0 {
		return stats, nil
	}

	stats.TotalDays = len(dates)
	stats.FirstDate = &dates[0]

	// 计算连续天数（从今天往回数）
	today := time.Now().Truncate(24 * time.Hour)
	consecutive := 0

	// 创建日期集合便于查找
	dateSet := make(map[string]bool)
	for _, d := range dates {
		dateSet[d.Format("2006-01-02")] = true
	}

	// 从今天开始往回数连续天数
	for i := 0; i <= 365; i++ { // 最多检查一年
		checkDate := today.AddDate(0, 0, -i).Format("2006-01-02")
		if dateSet[checkDate] {
			consecutive++
		} else if consecutive > 0 {
			break // 连续中断
		}
	}

	stats.ConsecutiveDays = consecutive
	return stats, nil
}

// calculateTokenUsage 计算 Token 消耗
func (s *Service) calculateTokenUsage(ctx context.Context, tenantID, userID string, start, end time.Time) (*tokenStats, error) {
	stats := &tokenStats{}

	query := s.db.WithContext(ctx).
		Table("ai_call_logs").
		Select("COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost), 0) as total_cost").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end)

	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Scan(stats).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// countActiveDays 统计活跃天数
func (s *Service) countActiveDays(ctx context.Context, tenantID, userID string, start, end time.Time) (int, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Table("workspace_file_versions").
		Select("COUNT(DISTINCT DATE(created_at))").
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Scan(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// aggregateWeeklyOutput 聚合周产出
func (s *Service) aggregateWeeklyOutput(daily []DailyWordCount) []WeeklyWordCount {
	if len(daily) == 0 {
		return nil
	}

	weeklyMap := make(map[string]*WeeklyWordCount)

	for _, d := range daily {
		date, _ := time.Parse("2006-01-02", d.Date)
		// 找到该周的周一
		weekday := int(date.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := date.AddDate(0, 0, -(weekday - 1))
		sunday := monday.AddDate(0, 0, 6)

		key := monday.Format("2006-01-02")
		if _, exists := weeklyMap[key]; !exists {
			weeklyMap[key] = &WeeklyWordCount{
				WeekStart: monday.Format("2006-01-02"),
				WeekEnd:   sunday.Format("2006-01-02"),
			}
		}
		weeklyMap[key].WordCount += d.WordCount
		weeklyMap[key].Documents += d.Documents
	}

	// 转为切片并排序
	result := make([]WeeklyWordCount, 0, len(weeklyMap))
	for _, w := range weeklyMap {
		result = append(result, *w)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].WeekStart < result[j].WeekStart
	})

	return result
}

// countWords 统计中英文字数（复用 text_statistics_tool 逻辑）
func countWords(text string) (chineseChars, englishWords int) {
	// 统计中文字符
	for _, r := range text {
		if isChinese(r) {
			chineseChars++
		}
	}

	// 统计英文单词
	re := regexp.MustCompile(`[\x{4e00}-\x{9fa5}]`)
	englishText := re.ReplaceAllString(text, "")

	words := regexp.MustCompile(`\s+`).Split(englishText, -1)
	for _, word := range words {
		hasLetter := false
		for _, r := range word {
			if unicode.IsLetter(r) {
				hasLetter = true
				break
			}
		}
		if hasLetter {
			englishWords++
		}
	}

	return
}

// isChinese 判断是否为中文字符
func isChinese(r rune) bool {
	return (r >= 0x4e00 && r <= 0x9fff) ||
		(r >= 0x3400 && r <= 0x4dbf) ||
		(r >= 0x20000 && r <= 0x2a6df)
}
