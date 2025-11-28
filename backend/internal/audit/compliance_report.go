package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ComplianceReportService 合规报告生成服务
type ComplianceReportService struct {
	logStore    LogStore
	userStore   UserStore
	configStore ConfigStore
}

// UserStore 用户存储接口
type UserStore interface {
	GetUserInfo(ctx context.Context, userID string) (*UserInfo, error)
	ListActiveUsers(ctx context.Context, since time.Time) ([]UserInfo, error)
}

// ConfigStore 配置存储接口
type ConfigStore interface {
	GetSecurityConfig(ctx context.Context) (*SecurityConfig, error)
	GetComplianceConfig(ctx context.Context) (*ComplianceConfig, error)
}

// UserInfo 用户信息
type UserInfo struct {
	ID        string
	Email     string
	Name      string
	Role      string
	Status    string
	LastLogin *time.Time
	MFAEnabled bool
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	PasswordPolicy      PasswordPolicy
	SessionTimeout      time.Duration
	MFARequired         bool
	IPWhitelistEnabled  bool
	RateLimitEnabled    bool
}

// PasswordPolicy 密码策略
type PasswordPolicy struct {
	MinLength        int
	RequireUppercase bool
	RequireLowercase bool
	RequireDigit     bool
	RequireSpecial   bool
	MaxAge           int // 天数
}

// ComplianceConfig 合规配置
type ComplianceConfig struct {
	DataRetentionDays  int
	AuditLogEnabled    bool
	EncryptionEnabled  bool
	GDPR               bool
	HIPAA              bool
}

// NewComplianceReportService 创建合规报告服务
func NewComplianceReportService(logStore LogStore, userStore UserStore, configStore ConfigStore) *ComplianceReportService {
	return &ComplianceReportService{
		logStore:    logStore,
		userStore:   userStore,
		configStore: configStore,
	}
}

// ComplianceReport 合规报告
type ComplianceReport struct {
	ReportID      string                `json:"report_id"`
	ReportType    string                `json:"report_type"`
	GeneratedAt   time.Time             `json:"generated_at"`
	Period        ReportPeriod          `json:"period"`
	Summary       ReportSummary         `json:"summary"`
	SecurityCheck SecurityCheckResult   `json:"security_check"`
	AccessAudit   AccessAuditResult     `json:"access_audit"`
	DataPrivacy   DataPrivacyResult     `json:"data_privacy"`
	Incidents     []SecurityIncident    `json:"incidents,omitempty"`
	Recommendations []string            `json:"recommendations,omitempty"`
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Type      string    `json:"type"` // daily / weekly / monthly / quarterly
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalUsers        int     `json:"total_users"`
	ActiveUsers       int     `json:"active_users"`
	TotalLogins       int64   `json:"total_logins"`
	FailedLogins      int64   `json:"failed_logins"`
	DataAccessEvents  int64   `json:"data_access_events"`
	SecurityIncidents int     `json:"security_incidents"`
	ComplianceScore   float64 `json:"compliance_score"`
	RiskLevel         string  `json:"risk_level"`
}

// SecurityCheckResult 安全检查结果
type SecurityCheckResult struct {
	PasswordPolicyCompliant bool               `json:"password_policy_compliant"`
	MFAAdoption             float64            `json:"mfa_adoption"`
	InactiveAccounts        int                `json:"inactive_accounts"`
	PrivilegedUsers         int                `json:"privileged_users"`
	SessionSecurityOK       bool               `json:"session_security_ok"`
	EncryptionEnabled       bool               `json:"encryption_enabled"`
	Findings                []SecurityFinding  `json:"findings,omitempty"`
}

// SecurityFinding 安全发现
type SecurityFinding struct {
	Severity    string `json:"severity"` // high / medium / low
	Category    string `json:"category"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
}

// AccessAuditResult 访问审计结果
type AccessAuditResult struct {
	TotalAccessEvents     int64               `json:"total_access_events"`
	UniqueUsers           int                 `json:"unique_users"`
	TopAccessedResources  []ResourceAccess    `json:"top_accessed_resources"`
	UnusualAccessPatterns []UnusualAccess     `json:"unusual_access_patterns,omitempty"`
	OffHoursAccess        int64               `json:"off_hours_access"`
	GeoAnomalies          int                 `json:"geo_anomalies"`
}

// ResourceAccess 资源访问统计
type ResourceAccess struct {
	Resource    string `json:"resource"`
	AccessCount int64  `json:"access_count"`
	UniqueUsers int    `json:"unique_users"`
}

// UnusualAccess 异常访问
type UnusualAccess struct {
	UserID      string    `json:"user_id"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	RiskScore   float64   `json:"risk_score"`
}

// DataPrivacyResult 数据隐私结果
type DataPrivacyResult struct {
	PersonalDataAccess    int64  `json:"personal_data_access"`
	DataExports           int64  `json:"data_exports"`
	ConsentRecorded       bool   `json:"consent_recorded"`
	DataRetentionCompliant bool  `json:"data_retention_compliant"`
	EncryptedAtRest       bool   `json:"encrypted_at_rest"`
	EncryptedInTransit    bool   `json:"encrypted_in_transit"`
}

// SecurityIncident 安全事件
type SecurityIncident struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
	Resolution  string    `json:"resolution,omitempty"`
}

// GenerateReport 生成合规报告
func (s *ComplianceReportService) GenerateReport(ctx context.Context, period ReportPeriod) (*ComplianceReport, error) {
	report := &ComplianceReport{
		ReportID:    fmt.Sprintf("CR_%d", time.Now().UnixNano()),
		ReportType:  "compliance",
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// 生成摘要
	summary, err := s.generateSummary(ctx, period)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}
	report.Summary = *summary

	// 安全检查
	securityCheck, err := s.performSecurityCheck(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to perform security check: %w", err)
	}
	report.SecurityCheck = *securityCheck

	// 访问审计
	accessAudit, err := s.performAccessAudit(ctx, period)
	if err != nil {
		return nil, fmt.Errorf("failed to perform access audit: %w", err)
	}
	report.AccessAudit = *accessAudit

	// 数据隐私检查
	dataPrivacy, err := s.checkDataPrivacy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check data privacy: %w", err)
	}
	report.DataPrivacy = *dataPrivacy

	// 获取安全事件
	incidents, _ := s.getSecurityIncidents(ctx, period)
	report.Incidents = incidents

	// 生成建议
	report.Recommendations = s.generateRecommendations(report)

	return report, nil
}

func (s *ComplianceReportService) generateSummary(ctx context.Context, period ReportPeriod) (*ReportSummary, error) {
	summary := &ReportSummary{}

	// 获取活跃用户
	users, err := s.userStore.ListActiveUsers(ctx, period.StartDate)
	if err == nil {
		summary.ActiveUsers = len(users)
	}

	// 获取登录统计
	loginFilter := LogFilter{
		StartTime: &period.StartDate,
		EndTime:   &period.EndDate,
		Action:    "login",
	}
	if logs, err := s.logStore.QueryLogs(ctx, loginFilter); err == nil {
		summary.TotalLogins = int64(len(logs))
		for _, log := range logs {
			if log.Status == "failed" {
				summary.FailedLogins++
			}
		}
	}

	// 获取数据访问事件
	accessFilter := LogFilter{
		StartTime: &period.StartDate,
		EndTime:   &period.EndDate,
		Action:    "data_access",
	}
	if logs, err := s.logStore.QueryLogs(ctx, accessFilter); err == nil {
		summary.DataAccessEvents = int64(len(logs))
	}

	// 计算合规分数
	summary.ComplianceScore = s.calculateComplianceScore(summary)
	summary.RiskLevel = s.calculateRiskLevel(summary.ComplianceScore)

	return summary, nil
}

func (s *ComplianceReportService) performSecurityCheck(ctx context.Context) (*SecurityCheckResult, error) {
	result := &SecurityCheckResult{
		Findings: make([]SecurityFinding, 0),
	}

	// 获取安全配置
	secConfig, err := s.configStore.GetSecurityConfig(ctx)
	if err == nil {
		result.PasswordPolicyCompliant = s.checkPasswordPolicy(secConfig.PasswordPolicy)
		result.SessionSecurityOK = secConfig.SessionTimeout <= 30*time.Minute
		result.EncryptionEnabled = true // 假设已启用
	}

	// 获取用户统计
	users, err := s.userStore.ListActiveUsers(ctx, time.Now().AddDate(0, -3, 0))
	if err == nil {
		mfaCount := 0
		privilegedCount := 0
		for _, user := range users {
			if user.MFAEnabled {
				mfaCount++
			}
			if user.Role == "admin" || user.Role == "owner" {
				privilegedCount++
			}
		}
		if len(users) > 0 {
			result.MFAAdoption = float64(mfaCount) / float64(len(users)) * 100
		}
		result.PrivilegedUsers = privilegedCount
	}

	// 添加发现
	if result.MFAAdoption < 80 {
		result.Findings = append(result.Findings, SecurityFinding{
			Severity:    "medium",
			Category:    "authentication",
			Description: fmt.Sprintf("MFA adoption rate is %.1f%%, below 80%% threshold", result.MFAAdoption),
			Remediation: "Enable MFA for all users",
		})
	}

	return result, nil
}

func (s *ComplianceReportService) performAccessAudit(ctx context.Context, period ReportPeriod) (*AccessAuditResult, error) {
	result := &AccessAuditResult{
		TopAccessedResources: make([]ResourceAccess, 0),
	}

	// 获取访问日志
	filter := LogFilter{
		StartTime: &period.StartDate,
		EndTime:   &period.EndDate,
	}
	logs, err := s.logStore.QueryLogs(ctx, filter)
	if err != nil {
		return result, nil
	}

	result.TotalAccessEvents = int64(len(logs))

	// 统计资源访问
	resourceCount := make(map[string]int64)
	userSet := make(map[string]bool)
	for _, log := range logs {
		resourceCount[log.Resource]++
		userSet[log.UserID] = true

		// 检查非工作时间访问
		hour := log.Timestamp.Hour()
		if hour < 6 || hour > 22 {
			result.OffHoursAccess++
		}
	}

	result.UniqueUsers = len(userSet)

	// 排序获取 Top 资源
	type kv struct {
		Resource string
		Count    int64
	}
	var sorted []kv
	for r, c := range resourceCount {
		sorted = append(sorted, kv{r, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	for i, item := range sorted {
		if i >= 10 {
			break
		}
		result.TopAccessedResources = append(result.TopAccessedResources, ResourceAccess{
			Resource:    item.Resource,
			AccessCount: item.Count,
		})
	}

	return result, nil
}

func (s *ComplianceReportService) checkDataPrivacy(ctx context.Context) (*DataPrivacyResult, error) {
	result := &DataPrivacyResult{
		ConsentRecorded:       true,
		DataRetentionCompliant: true,
		EncryptedAtRest:       true,
		EncryptedInTransit:    true,
	}

	// 获取合规配置
	compConfig, err := s.configStore.GetComplianceConfig(ctx)
	if err == nil {
		result.DataRetentionCompliant = compConfig.DataRetentionDays <= 365
		result.EncryptedAtRest = compConfig.EncryptionEnabled
	}

	return result, nil
}

func (s *ComplianceReportService) getSecurityIncidents(ctx context.Context, period ReportPeriod) ([]SecurityIncident, error) {
	incidents := make([]SecurityIncident, 0)

	// 查找失败登录
	filter := LogFilter{
		StartTime: &period.StartDate,
		EndTime:   &period.EndDate,
		Action:    "login",
	}
	logs, err := s.logStore.QueryLogs(ctx, filter)
	if err != nil {
		return incidents, nil
	}

	// 检测异常
	failedByUser := make(map[string]int)
	for _, log := range logs {
		if log.Status == "failed" {
			failedByUser[log.UserID]++
		}
	}

	for userID, count := range failedByUser {
		if count >= 5 {
			incidents = append(incidents, SecurityIncident{
				ID:          fmt.Sprintf("INC_%d", time.Now().UnixNano()),
				Type:        "brute_force_attempt",
				Severity:    "high",
				Description: fmt.Sprintf("User %s had %d failed login attempts", userID, count),
				Timestamp:   time.Now(),
			})
		}
	}

	return incidents, nil
}

func (s *ComplianceReportService) checkPasswordPolicy(policy PasswordPolicy) bool {
	return policy.MinLength >= 8 &&
		policy.RequireUppercase &&
		policy.RequireLowercase &&
		policy.RequireDigit
}

func (s *ComplianceReportService) calculateComplianceScore(summary *ReportSummary) float64 {
	score := 100.0

	// 登录失败率扣分
	if summary.TotalLogins > 0 {
		failRate := float64(summary.FailedLogins) / float64(summary.TotalLogins)
		if failRate > 0.1 {
			score -= 10
		}
	}

	// 安全事件扣分
	score -= float64(summary.SecurityIncidents) * 5

	if score < 0 {
		score = 0
	}
	return score
}

func (s *ComplianceReportService) calculateRiskLevel(score float64) string {
	switch {
	case score >= 90:
		return "low"
	case score >= 70:
		return "medium"
	case score >= 50:
		return "high"
	default:
		return "critical"
	}
}

func (s *ComplianceReportService) generateRecommendations(report *ComplianceReport) []string {
	recommendations := make([]string, 0)

	if report.SecurityCheck.MFAAdoption < 100 {
		recommendations = append(recommendations, "Enable Multi-Factor Authentication (MFA) for all users")
	}

	if report.AccessAudit.OffHoursAccess > 100 {
		recommendations = append(recommendations, "Review and monitor off-hours access patterns")
	}

	if len(report.Incidents) > 0 {
		recommendations = append(recommendations, "Investigate and resolve all security incidents")
	}

	if report.Summary.ComplianceScore < 90 {
		recommendations = append(recommendations, "Review and strengthen security policies")
	}

	return recommendations
}

// ExportReport 导出报告
func (s *ComplianceReportService) ExportReport(report *ComplianceReport, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(report, "", "  ")
	case "markdown":
		return s.exportMarkdown(report)
	default:
		return json.MarshalIndent(report, "", "  ")
	}
}

func (s *ComplianceReportService) exportMarkdown(report *ComplianceReport) ([]byte, error) {
	var md strings.Builder

	md.WriteString("# Compliance Report\n\n")
	md.WriteString(fmt.Sprintf("**Report ID:** %s\n\n", report.ReportID))
	md.WriteString(fmt.Sprintf("**Generated:** %s\n\n", report.GeneratedAt.Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("**Period:** %s to %s\n\n", 
		report.Period.StartDate.Format("2006-01-02"),
		report.Period.EndDate.Format("2006-01-02")))

	md.WriteString("## Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Compliance Score:** %.1f%%\n", report.Summary.ComplianceScore))
	md.WriteString(fmt.Sprintf("- **Risk Level:** %s\n", report.Summary.RiskLevel))
	md.WriteString(fmt.Sprintf("- **Active Users:** %d\n", report.Summary.ActiveUsers))
	md.WriteString(fmt.Sprintf("- **Total Logins:** %d\n", report.Summary.TotalLogins))
	md.WriteString(fmt.Sprintf("- **Security Incidents:** %d\n\n", report.Summary.SecurityIncidents))

	md.WriteString("## Security Check\n\n")
	md.WriteString(fmt.Sprintf("- **MFA Adoption:** %.1f%%\n", report.SecurityCheck.MFAAdoption))
	md.WriteString(fmt.Sprintf("- **Privileged Users:** %d\n\n", report.SecurityCheck.PrivilegedUsers))

	if len(report.SecurityCheck.Findings) > 0 {
		md.WriteString("### Findings\n\n")
		for _, f := range report.SecurityCheck.Findings {
			md.WriteString(fmt.Sprintf("- [%s] %s: %s\n", f.Severity, f.Category, f.Description))
		}
		md.WriteString("\n")
	}

	if len(report.Recommendations) > 0 {
		md.WriteString("## Recommendations\n\n")
		for _, r := range report.Recommendations {
			md.WriteString(fmt.Sprintf("- %s\n", r))
		}
	}

	return []byte(md.String()), nil
}
