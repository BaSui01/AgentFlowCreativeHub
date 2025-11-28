package compliance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrVerificationNotFound = errors.New("认证记录不存在")
	ErrAlreadyVerified      = errors.New("用户已完成认证")
	ErrInvalidIDNumber      = errors.New("证件号码格式错误")
	ErrCheckNotFound        = errors.New("检查记录不存在")
)

// 敏感词列表（示例）
var defaultSensitiveWords = []string{
	"违禁词1", "违禁词2", // 实际使用时从数据库加载
}

// Service 合规服务
type Service struct {
	db             *gorm.DB
	sensitiveWords []string
}

// NewService 创建服务
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:             db,
		sensitiveWords: defaultSensitiveWords,
	}
}

// ============================================================================
// 实名认证
// ============================================================================

// SubmitVerification 提交实名认证
func (s *Service) SubmitVerification(ctx context.Context, req *SubmitVerificationRequest) (*UserVerification, error) {
	// 检查是否已认证
	var count int64
	s.db.WithContext(ctx).Model(&UserVerification{}).
		Where("user_id = ? AND status IN ?", req.UserID, []VerificationStatus{VerifyStatusPending, VerifyStatusApproved}).
		Count(&count)
	if count > 0 {
		return nil, ErrAlreadyVerified
	}

	// 验证证件号码格式
	if !validateIDNumber(req.IDNumber, req.VerifyType) {
		return nil, ErrInvalidIDNumber
	}

	verification := &UserVerification{
		ID:           uuid.New().String(),
		TenantID:     req.TenantID,
		UserID:       req.UserID,
		VerifyType:   req.VerifyType,
		Status:       VerifyStatusPending,
		RealName:     req.RealName,
		IDNumber:     hashIDNumber(req.IDNumber), // 加密存储
		IDNumberMask: maskIDNumber(req.IDNumber), // 脱敏显示
		Birthday:     req.Birthday,
		Gender:       req.Gender,
		IDFrontImage: req.IDFrontImage,
		IDBackImage:  req.IDBackImage,
		FaceImage:    req.FaceImage,
		CompanyName:  req.CompanyName,
		BusinessLicense: req.BusinessLicense,
	}

	if err := s.db.WithContext(ctx).Create(verification).Error; err != nil {
		return nil, err
	}

	return verification, nil
}

// GetVerification 获取认证详情
func (s *Service) GetVerification(ctx context.Context, verificationID string) (*UserVerification, error) {
	var v UserVerification
	if err := s.db.WithContext(ctx).Where("id = ?", verificationID).First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVerificationNotFound
		}
		return nil, err
	}
	return &v, nil
}

// GetUserVerification 获取用户的认证状态
func (s *Service) GetUserVerification(ctx context.Context, userID string) (*UserVerification, error) {
	var v UserVerification
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&v).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVerificationNotFound
		}
		return nil, err
	}
	return &v, nil
}

// ListVerifications 获取认证列表
func (s *Service) ListVerifications(ctx context.Context, tenantID string, status VerificationStatus, page, pageSize int) ([]UserVerification, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var verifications []UserVerification
	var total int64

	q := s.db.WithContext(ctx).Model(&UserVerification{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}

	q.Count(&total)
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&verifications)

	return verifications, total, nil
}

// ReviewVerification 审核实名认证
func (s *Service) ReviewVerification(ctx context.Context, req *ReviewVerificationRequest) error {
	now := time.Now()
	updates := map[string]interface{}{
		"reviewed_by": req.ReviewerID,
		"reviewed_at": now,
	}

	if req.Action == "approve" {
		updates["status"] = VerifyStatusApproved
		updates["verified_at"] = now
	} else if req.Action == "reject" {
		updates["status"] = VerifyStatusRejected
		updates["reject_reason"] = req.RejectReason
	} else {
		return fmt.Errorf("无效的操作: %s", req.Action)
	}

	result := s.db.WithContext(ctx).Model(&UserVerification{}).
		Where("id = ? AND status = ?", req.VerificationID, VerifyStatusPending).
		Updates(updates)

	if result.RowsAffected == 0 {
		return ErrVerificationNotFound
	}

	return result.Error
}

// ============================================================================
// 内容分级
// ============================================================================

// SetContentRating 设置内容分级
func (s *Service) SetContentRating(ctx context.Context, req *SetContentRatingRequest, raterID string) (*ContentRating, error) {
	// 查找已有分级记录
	var existing ContentRating
	err := s.db.WithContext(ctx).
		Where("content_id = ? AND content_type = ?", req.ContentID, req.ContentType).
		First(&existing).Error

	now := time.Now()
	
	if err == nil {
		// 更新已有记录
		updates := map[string]interface{}{
			"rating":        req.Rating,
			"rating_reason": req.RatingReason,
			"has_violence":  req.HasViolence,
			"has_sexual":    req.HasSexual,
			"has_drug":      req.HasDrug,
			"has_gambling":  req.HasGambling,
			"has_horror":    req.HasHorror,
			"has_political": req.HasPolitical,
			"rated_by":      raterID,
			"rated_at":      now,
			"is_auto_rated": false,
		}
		s.db.WithContext(ctx).Model(&ContentRating{}).Where("id = ?", existing.ID).Updates(updates)
		existing.Rating = req.Rating
		return &existing, nil
	}

	// 创建新记录
	rating := &ContentRating{
		ID:           uuid.New().String(),
		TenantID:     req.TenantID,
		ContentID:    req.ContentID,
		ContentType:  req.ContentType,
		Rating:       req.Rating,
		RatingReason: req.RatingReason,
		HasViolence:  req.HasViolence,
		HasSexual:    req.HasSexual,
		HasDrug:      req.HasDrug,
		HasGambling:  req.HasGambling,
		HasHorror:    req.HasHorror,
		HasPolitical: req.HasPolitical,
		IsAutoRated:  false,
		RatedBy:      raterID,
		RatedAt:      &now,
	}

	if err := s.db.WithContext(ctx).Create(rating).Error; err != nil {
		return nil, err
	}

	return rating, nil
}

// GetContentRating 获取内容分级
func (s *Service) GetContentRating(ctx context.Context, contentID, contentType string) (*ContentRating, error) {
	var rating ContentRating
	err := s.db.WithContext(ctx).
		Where("content_id = ? AND content_type = ?", contentID, contentType).
		First(&rating).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 未分级返回 nil
		}
		return nil, err
	}
	return &rating, nil
}

// AutoRateContent 自动内容分级
func (s *Service) AutoRateContent(ctx context.Context, tenantID, contentID, contentType, content string) (*ContentRating, error) {
	// 简单的自动分级逻辑
	rating := RatingAll
	hasViolence := false
	hasSexual := false

	// 检测暴力内容关键词
	violenceKeywords := []string{"杀", "血", "死", "暴力", "伤害"}
	for _, kw := range violenceKeywords {
		if strings.Contains(content, kw) {
			hasViolence = true
			rating = RatingTeen
			break
		}
	}

	// 检测敏感内容
	for _, word := range s.sensitiveWords {
		if strings.Contains(content, word) {
			hasSexual = true
			rating = RatingMature
			break
		}
	}

	now := time.Now()
	ratingRecord := &ContentRating{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		ContentID:   contentID,
		ContentType: contentType,
		Rating:      rating,
		HasViolence: hasViolence,
		HasSexual:   hasSexual,
		IsAutoRated: true,
		RatedAt:     &now,
	}

	// 使用 upsert
	s.db.WithContext(ctx).Where("content_id = ? AND content_type = ?", contentID, contentType).
		Assign(ratingRecord).FirstOrCreate(ratingRecord)

	return ratingRecord, nil
}

// ============================================================================
// 合规检查
// ============================================================================

// RunComplianceCheck 执行合规检查
func (s *Service) RunComplianceCheck(ctx context.Context, req *RunComplianceCheckRequest) (*ComplianceCheck, error) {
	now := time.Now()
	
	check := &ComplianceCheck{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		ContentID:   req.ContentID,
		ContentType: req.ContentType,
		CheckType:   req.CheckType,
		Status:      CheckStatusPending,
		CheckedBy:   "system",
		CheckedAt:   &now,
	}

	// 执行检查
	issues := []string{}
	sensitiveFound := []string{}
	score := 100.0

	// 敏感词检测
	if req.CheckType == "text" || req.CheckType == "all" {
		for _, word := range s.sensitiveWords {
			if strings.Contains(req.Content, word) {
				sensitiveFound = append(sensitiveFound, word)
				score -= 10
			}
		}
	}

	// 确定状态
	if len(sensitiveFound) > 0 {
		sensitiveJSON, _ := json.Marshal(sensitiveFound)
		check.SensitiveWords = string(sensitiveJSON)
		issues = append(issues, fmt.Sprintf("发现%d个敏感词", len(sensitiveFound)))
	}

	if score >= 80 {
		check.Status = CheckStatusPassed
		check.RiskLevel = "low"
	} else if score >= 60 {
		check.Status = CheckStatusWarning
		check.RiskLevel = "medium"
	} else {
		check.Status = CheckStatusFailed
		check.RiskLevel = "high"
	}

	check.Score = score
	if len(issues) > 0 {
		issuesJSON, _ := json.Marshal(issues)
		check.Issues = string(issuesJSON)
	}

	if err := s.db.WithContext(ctx).Create(check).Error; err != nil {
		return nil, err
	}

	// 如果检查失败，创建风险提示
	if check.Status == CheckStatusFailed {
		s.CreateRiskAlert(ctx, req.TenantID, req.ContentID, "", "content", "high",
			"内容合规检查未通过", fmt.Sprintf("内容得分: %.0f，存在合规风险", score), "建议修改内容后重新提交")
	}

	return check, nil
}

// GetComplianceCheck 获取检查详情
func (s *Service) GetComplianceCheck(ctx context.Context, checkID string) (*ComplianceCheck, error) {
	var check ComplianceCheck
	if err := s.db.WithContext(ctx).Where("id = ?", checkID).First(&check).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCheckNotFound
		}
		return nil, err
	}
	return &check, nil
}

// ListComplianceChecks 获取检查列表
func (s *Service) ListComplianceChecks(ctx context.Context, tenantID string, status CheckStatus, page, pageSize int) ([]ComplianceCheck, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var checks []ComplianceCheck
	var total int64

	q := s.db.WithContext(ctx).Model(&ComplianceCheck{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		q = q.Where("status = ?", status)
	}

	q.Count(&total)
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&checks)

	return checks, total, nil
}

// ============================================================================
// 版权保护
// ============================================================================

// RegisterCopyright 登记版权
func (s *Service) RegisterCopyright(ctx context.Context, tenantID, contentID, userID string, copyrightType, author, declaration, licenseType string) (*CopyrightRecord, error) {
	now := time.Now()
	record := &CopyrightRecord{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		ContentID:     contentID,
		UserID:        userID,
		Status:        CopyrightPending,
		CopyrightType: copyrightType,
		Author:        author,
		PublishDate:   &now,
		Declaration:   declaration,
		LicenseType:   licenseType,
	}

	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, err
	}

	return record, nil
}

// GetCopyrightRecord 获取版权记录
func (s *Service) GetCopyrightRecord(ctx context.Context, contentID string) (*CopyrightRecord, error) {
	var record CopyrightRecord
	err := s.db.WithContext(ctx).Where("content_id = ?", contentID).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// UpdateCopyrightStatus 更新版权状态
func (s *Service) UpdateCopyrightStatus(ctx context.Context, recordID string, status CopyrightStatus) error {
	return s.db.WithContext(ctx).Model(&CopyrightRecord{}).
		Where("id = ?", recordID).
		Update("status", status).Error
}

// ============================================================================
// 风险提示
// ============================================================================

// CreateRiskAlert 创建风险提示
func (s *Service) CreateRiskAlert(ctx context.Context, tenantID, contentID, userID, alertType, riskLevel, title, description, suggestion string) (*RiskAlert, error) {
	alert := &RiskAlert{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		ContentID:   contentID,
		UserID:      userID,
		AlertType:   alertType,
		RiskLevel:   riskLevel,
		Title:       title,
		Description: description,
		Suggestion:  suggestion,
	}

	if err := s.db.WithContext(ctx).Create(alert).Error; err != nil {
		return nil, err
	}

	return alert, nil
}

// ListRiskAlerts 获取风险提示列表
func (s *Service) ListRiskAlerts(ctx context.Context, tenantID, userID string, unresolvedOnly bool, page, pageSize int) ([]RiskAlert, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var alerts []RiskAlert
	var total int64

	q := s.db.WithContext(ctx).Model(&RiskAlert{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if unresolvedOnly {
		q = q.Where("is_resolved = ?", false)
	}

	q.Count(&total)
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&alerts)

	return alerts, total, nil
}

// ResolveRiskAlert 解决风险提示
func (s *Service) ResolveRiskAlert(ctx context.Context, alertID, resolverID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&RiskAlert{}).
		Where("id = ?", alertID).
		Updates(map[string]interface{}{
			"is_resolved": true,
			"resolved_at": now,
			"resolved_by": resolverID,
		}).Error
}

// ============================================================================
// 合规报告
// ============================================================================

// GenerateComplianceReport 生成合规报告
func (s *Service) GenerateComplianceReport(ctx context.Context, req *GenerateReportRequest, generatorID string) (*ComplianceReport, error) {
	report := &ComplianceReport{
		ID:          uuid.New().String(),
		TenantID:    req.TenantID,
		ReportNo:    generateReportNo(),
		ReportType:  req.ReportType,
		PeriodStart: req.PeriodStart,
		PeriodEnd:   req.PeriodEnd,
		GeneratedBy: generatorID,
	}

	// 统计数据
	// 内容检查统计
	s.db.WithContext(ctx).Model(&ComplianceCheck{}).
		Where("tenant_id = ? AND created_at BETWEEN ? AND ?", req.TenantID, req.PeriodStart, req.PeriodEnd).
		Count(&report.CheckedContent)

	s.db.WithContext(ctx).Model(&ComplianceCheck{}).
		Where("tenant_id = ? AND status = ? AND created_at BETWEEN ? AND ?", req.TenantID, CheckStatusPassed, req.PeriodStart, req.PeriodEnd).
		Count(&report.PassedContent)

	s.db.WithContext(ctx).Model(&ComplianceCheck{}).
		Where("tenant_id = ? AND status = ? AND created_at BETWEEN ? AND ?", req.TenantID, CheckStatusFailed, req.PeriodStart, req.PeriodEnd).
		Count(&report.FailedContent)

	// 认证统计
	s.db.WithContext(ctx).Model(&UserVerification{}).
		Where("tenant_id = ? AND created_at BETWEEN ? AND ?", req.TenantID, req.PeriodStart, req.PeriodEnd).
		Count(&report.TotalVerifications)

	s.db.WithContext(ctx).Model(&UserVerification{}).
		Where("tenant_id = ? AND status = ? AND created_at BETWEEN ? AND ?", req.TenantID, VerifyStatusApproved, req.PeriodStart, req.PeriodEnd).
		Count(&report.ApprovedVerifications)

	// 计算合规率
	if report.CheckedContent > 0 {
		report.ComplianceRate = float64(report.PassedContent) / float64(report.CheckedContent) * 100
	}

	if err := s.db.WithContext(ctx).Create(report).Error; err != nil {
		return nil, err
	}

	return report, nil
}

// GetComplianceReport 获取合规报告
func (s *Service) GetComplianceReport(ctx context.Context, reportID string) (*ComplianceReport, error) {
	var report ComplianceReport
	if err := s.db.WithContext(ctx).Where("id = ?", reportID).First(&report).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

// ListComplianceReports 获取报告列表
func (s *Service) ListComplianceReports(ctx context.Context, tenantID string, page, pageSize int) ([]ComplianceReport, int64, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}

	var reports []ComplianceReport
	var total int64

	q := s.db.WithContext(ctx).Model(&ComplianceReport{}).Where("tenant_id = ?", tenantID)
	q.Count(&total)
	q.Order("generated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&reports)

	return reports, total, nil
}

// ============================================================================
// 统计
// ============================================================================

// GetComplianceStats 获取合规统计
func (s *Service) GetComplianceStats(ctx context.Context, tenantID string) (*ComplianceStats, error) {
	stats := &ComplianceStats{TenantID: tenantID}

	// 认证统计
	s.db.WithContext(ctx).Model(&UserVerification{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalVerifications)
	s.db.WithContext(ctx).Model(&UserVerification{}).Where("tenant_id = ? AND status = ?", tenantID, VerifyStatusPending).Count(&stats.PendingVerifications)
	s.db.WithContext(ctx).Model(&UserVerification{}).Where("tenant_id = ? AND status = ?", tenantID, VerifyStatusApproved).Count(&stats.ApprovedVerifications)

	// 检查统计
	s.db.WithContext(ctx).Model(&ComplianceCheck{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalChecks)
	s.db.WithContext(ctx).Model(&ComplianceCheck{}).Where("tenant_id = ? AND status = ?", tenantID, CheckStatusPassed).Count(&stats.PassedChecks)
	s.db.WithContext(ctx).Model(&ComplianceCheck{}).Where("tenant_id = ? AND status = ?", tenantID, CheckStatusFailed).Count(&stats.FailedChecks)

	if stats.TotalChecks > 0 {
		stats.ComplianceRate = float64(stats.PassedChecks) / float64(stats.TotalChecks) * 100
	}

	// 风险提示统计
	s.db.WithContext(ctx).Model(&RiskAlert{}).Where("tenant_id = ?", tenantID).Count(&stats.TotalAlerts)
	s.db.WithContext(ctx).Model(&RiskAlert{}).Where("tenant_id = ? AND is_resolved = ?", tenantID, false).Count(&stats.UnresolvedAlerts)

	return stats, nil
}

// AutoMigrate 自动迁移表结构
func (s *Service) AutoMigrate() error {
	return s.db.AutoMigrate(
		&UserVerification{},
		&ContentRating{},
		&ComplianceCheck{},
		&CopyrightRecord{},
		&RiskAlert{},
		&ComplianceReport{},
	)
}

// ============================================================================
// 辅助函数
// ============================================================================

func validateIDNumber(idNumber, verifyType string) bool {
	switch verifyType {
	case VerifyTypeIDCard:
		// 中国身份证 18位
		matched, _ := regexp.MatchString(`^\d{17}[\dXx]$`, idNumber)
		return matched
	case VerifyTypePassport:
		// 护照号码
		return len(idNumber) >= 5 && len(idNumber) <= 20
	case VerifyTypeBusiness:
		// 统一社会信用代码 18位
		return len(idNumber) == 18
	}
	return true
}

func hashIDNumber(idNumber string) string {
	hash := sha256.Sum256([]byte(idNumber))
	return hex.EncodeToString(hash[:])
}

func maskIDNumber(idNumber string) string {
	if len(idNumber) <= 6 {
		return "****"
	}
	return idNumber[:3] + strings.Repeat("*", len(idNumber)-6) + idNumber[len(idNumber)-3:]
}

func generateReportNo() string {
	return fmt.Sprintf("CR%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
}
