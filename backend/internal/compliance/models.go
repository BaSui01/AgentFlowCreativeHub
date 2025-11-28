package compliance

import (
	"time"
)

// ============================================================================
// 实名认证
// ============================================================================

// VerificationStatus 认证状态
type VerificationStatus string

const (
	VerifyStatusPending  VerificationStatus = "pending"  // 待审核
	VerifyStatusApproved VerificationStatus = "approved" // 已通过
	VerifyStatusRejected VerificationStatus = "rejected" // 已拒绝
	VerifyStatusExpired  VerificationStatus = "expired"  // 已过期
)

// VerificationType 认证类型
const (
	VerifyTypeIDCard   = "id_card"   // 身份证
	VerifyTypePassport = "passport"  // 护照
	VerifyTypeBusiness = "business"  // 企业认证
)

// UserVerification 用户实名认证
type UserVerification struct {
	ID        string             `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string             `json:"tenantId" gorm:"type:uuid;not null;index"`
	UserID    string             `json:"userId" gorm:"type:uuid;not null;uniqueIndex"`

	// 认证类型
	VerifyType string             `json:"verifyType" gorm:"size:20;not null"`
	Status     VerificationStatus `json:"status" gorm:"size:20;not null;default:pending;index"`

	// 个人信息
	RealName    string     `json:"realName" gorm:"size:100"`
	IDNumber    string     `json:"idNumber" gorm:"size:50"`       // 加密存储
	IDNumberMask string    `json:"idNumberMask" gorm:"size:50"`   // 脱敏显示
	Birthday    *time.Time `json:"birthday"`
	Gender      string     `json:"gender" gorm:"size:10"`

	// 证件信息
	IDFrontImage string     `json:"idFrontImage" gorm:"size:500"` // 证件正面
	IDBackImage  string     `json:"idBackImage" gorm:"size:500"`  // 证件背面
	FaceImage    string     `json:"faceImage" gorm:"size:500"`    // 人脸照片
	ValidFrom    *time.Time `json:"validFrom"`                    // 证件有效期开始
	ValidTo      *time.Time `json:"validTo"`                      // 证件有效期结束

	// 企业认证（可选）
	CompanyName    string `json:"companyName" gorm:"size:200"`
	BusinessLicense string `json:"businessLicense" gorm:"size:50"`
	LegalPerson    string `json:"legalPerson" gorm:"size:100"`

	// 审核信息
	ReviewedBy   string     `json:"reviewedBy" gorm:"type:uuid"`
	ReviewedAt   *time.Time `json:"reviewedAt"`
	RejectReason string     `json:"rejectReason" gorm:"size:500"`

	// 第三方验证
	ThirdPartyResult string `json:"thirdPartyResult" gorm:"type:text"` // 第三方验证结果 JSON
	VerifiedAt       *time.Time `json:"verifiedAt"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (UserVerification) TableName() string {
	return "user_verifications"
}

// ============================================================================
// 内容年龄分级
// ============================================================================

// AgeRating 年龄分级
type AgeRating string

const (
	RatingAll    AgeRating = "all"    // 全年龄
	RatingTeen   AgeRating = "teen"   // 13+
	RatingMature AgeRating = "mature" // 18+
	RatingAdult  AgeRating = "adult"  // 仅成人
)

// ContentRating 内容分级记录
type ContentRating struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	ContentID string    `json:"contentId" gorm:"type:uuid;not null;index"` // 作品ID
	ContentType string  `json:"contentType" gorm:"size:50;not null"`       // work, chapter, etc.

	// 分级信息
	Rating       AgeRating `json:"rating" gorm:"size:20;not null;default:all"`
	RatingReason string    `json:"ratingReason" gorm:"size:500"`
	
	// 内容标签
	HasViolence   bool `json:"hasViolence" gorm:"default:false"`
	HasSexual     bool `json:"hasSexual" gorm:"default:false"`
	HasDrug       bool `json:"hasDrug" gorm:"default:false"`
	HasGambling   bool `json:"hasGambling" gorm:"default:false"`
	HasHorror     bool `json:"hasHorror" gorm:"default:false"`
	HasPolitical  bool `json:"hasPolitical" gorm:"default:false"`

	// 分级方式
	IsAutoRated  bool       `json:"isAutoRated" gorm:"default:false"` // 是否自动分级
	RatedBy      string     `json:"ratedBy" gorm:"type:uuid"`
	RatedAt      *time.Time `json:"ratedAt"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ContentRating) TableName() string {
	return "content_ratings"
}

// ============================================================================
// 合规检查
// ============================================================================

// CheckStatus 检查状态
type CheckStatus string

const (
	CheckStatusPending  CheckStatus = "pending"  // 待检查
	CheckStatusPassed   CheckStatus = "passed"   // 通过
	CheckStatusFailed   CheckStatus = "failed"   // 未通过
	CheckStatusWarning  CheckStatus = "warning"  // 警告
)

// ComplianceCheck 合规检查记录
type ComplianceCheck struct {
	ID          string      `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string      `json:"tenantId" gorm:"type:uuid;not null;index"`
	ContentID   string      `json:"contentId" gorm:"type:uuid;not null;index"`
	ContentType string      `json:"contentType" gorm:"size:50;not null"`

	// 检查类型
	CheckType string      `json:"checkType" gorm:"size:50;not null"` // text, image, copyright
	Status    CheckStatus `json:"status" gorm:"size:20;not null;default:pending;index"`

	// 检查结果
	Score        float64 `json:"score" gorm:"type:decimal(5,2)"`       // 合规分数 0-100
	RiskLevel    string  `json:"riskLevel" gorm:"size:20"`             // low, medium, high
	Issues       string  `json:"issues" gorm:"type:jsonb"`             // 发现的问题 JSON
	Suggestions  string  `json:"suggestions" gorm:"type:text"`         // 改进建议
	
	// 敏感词检测
	SensitiveWords string `json:"sensitiveWords" gorm:"type:jsonb"`    // 命中的敏感词

	// 检查详情
	CheckedBy    string     `json:"checkedBy" gorm:"size:50"`          // system, manual, ai
	CheckedAt    *time.Time `json:"checkedAt"`
	ReviewedBy   string     `json:"reviewedBy" gorm:"type:uuid"`       // 人工复核
	ReviewedAt   *time.Time `json:"reviewedAt"`
	ReviewNote   string     `json:"reviewNote" gorm:"size:500"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (ComplianceCheck) TableName() string {
	return "compliance_checks"
}

// ============================================================================
// 版权保护
// ============================================================================

// CopyrightStatus 版权状态
type CopyrightStatus string

const (
	CopyrightPending   CopyrightStatus = "pending"   // 待确认
	CopyrightOriginal  CopyrightStatus = "original"  // 原创
	CopyrightLicensed  CopyrightStatus = "licensed"  // 已授权
	CopyrightDisputed  CopyrightStatus = "disputed"  // 争议中
	CopyrightInfringe  CopyrightStatus = "infringe"  // 侵权
)

// CopyrightRecord 版权记录
type CopyrightRecord struct {
	ID          string          `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string          `json:"tenantId" gorm:"type:uuid;not null;index"`
	ContentID   string          `json:"contentId" gorm:"type:uuid;not null;index"`
	UserID      string          `json:"userId" gorm:"type:uuid;not null;index"`

	// 版权信息
	Status        CopyrightStatus `json:"status" gorm:"size:20;not null;default:pending"`
	CopyrightType string          `json:"copyrightType" gorm:"size:50"` // original, adaptation, translation
	Author        string          `json:"author" gorm:"size:200"`
	PublishDate   *time.Time      `json:"publishDate"`
	
	// 版权声明
	Declaration   string `json:"declaration" gorm:"type:text"`
	LicenseType   string `json:"licenseType" gorm:"size:50"` // CC-BY, CC-BY-NC, etc.
	
	// 原作信息（改编/翻译时）
	OriginalTitle  string `json:"originalTitle" gorm:"size:200"`
	OriginalAuthor string `json:"originalAuthor" gorm:"size:200"`
	OriginalSource string `json:"originalSource" gorm:"size:500"`
	AuthorizationDoc string `json:"authorizationDoc" gorm:"size:500"` // 授权文件

	// 侵权检测
	SimilarityScore float64    `json:"similarityScore" gorm:"type:decimal(5,2)"`
	SimilarWorks    string     `json:"similarWorks" gorm:"type:jsonb"` // 相似作品列表
	LastCheckedAt   *time.Time `json:"lastCheckedAt"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (CopyrightRecord) TableName() string {
	return "copyright_records"
}

// ============================================================================
// 法律风险提示
// ============================================================================

// RiskAlert 风险提示
type RiskAlert struct {
	ID        string `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string `json:"tenantId" gorm:"type:uuid;not null;index"`
	ContentID string `json:"contentId" gorm:"type:uuid;index"`
	UserID    string `json:"userId" gorm:"type:uuid;index"`

	// 风险信息
	AlertType   string `json:"alertType" gorm:"size:50;not null"`   // legal, copyright, content, privacy
	RiskLevel   string `json:"riskLevel" gorm:"size:20;not null"`   // low, medium, high, critical
	Title       string `json:"title" gorm:"size:200;not null"`
	Description string `json:"description" gorm:"type:text"`
	Suggestion  string `json:"suggestion" gorm:"type:text"`
	LegalBasis  string `json:"legalBasis" gorm:"size:500"`          // 法律依据

	// 状态
	IsRead      bool       `json:"isRead" gorm:"default:false"`
	IsResolved  bool       `json:"isResolved" gorm:"default:false"`
	ResolvedAt  *time.Time `json:"resolvedAt"`
	ResolvedBy  string     `json:"resolvedBy" gorm:"type:uuid"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}

func (RiskAlert) TableName() string {
	return "risk_alerts"
}

// ============================================================================
// 合规报告
// ============================================================================

// ComplianceReport 合规报告
type ComplianceReport struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string    `json:"tenantId" gorm:"type:uuid;not null;index"`
	ReportNo  string    `json:"reportNo" gorm:"size:50;not null;uniqueIndex"`
	
	// 报告周期
	ReportType  string     `json:"reportType" gorm:"size:20;not null"` // daily, weekly, monthly, quarterly
	PeriodStart time.Time  `json:"periodStart" gorm:"not null"`
	PeriodEnd   time.Time  `json:"periodEnd" gorm:"not null"`
	
	// 统计数据
	TotalContent      int64   `json:"totalContent" gorm:"default:0"`
	CheckedContent    int64   `json:"checkedContent" gorm:"default:0"`
	PassedContent     int64   `json:"passedContent" gorm:"default:0"`
	FailedContent     int64   `json:"failedContent" gorm:"default:0"`
	ComplianceRate    float64 `json:"complianceRate" gorm:"type:decimal(5,2)"`
	
	TotalReports      int64   `json:"totalReports" gorm:"default:0"`      // 举报数
	ProcessedReports  int64   `json:"processedReports" gorm:"default:0"`
	OfflineContent    int64   `json:"offlineContent" gorm:"default:0"`    // 下架数
	
	TotalVerifications int64  `json:"totalVerifications" gorm:"default:0"` // 认证数
	ApprovedVerifications int64 `json:"approvedVerifications" gorm:"default:0"`
	
	// 详细数据
	Details string `json:"details" gorm:"type:jsonb"`
	
	// 生成信息
	GeneratedAt time.Time `json:"generatedAt" gorm:"autoCreateTime"`
	GeneratedBy string    `json:"generatedBy" gorm:"type:uuid"`
	FileURL     string    `json:"fileUrl" gorm:"size:500"` // PDF报告链接
}

func (ComplianceReport) TableName() string {
	return "compliance_reports"
}

// ============================================================================
// 请求结构
// ============================================================================

// SubmitVerificationRequest 提交实名认证请求
type SubmitVerificationRequest struct {
	TenantID     string     `json:"tenantId"`
	UserID       string     `json:"userId"`
	VerifyType   string     `json:"verifyType" binding:"required"`
	RealName     string     `json:"realName" binding:"required"`
	IDNumber     string     `json:"idNumber" binding:"required"`
	Birthday     *time.Time `json:"birthday"`
	Gender       string     `json:"gender"`
	IDFrontImage string     `json:"idFrontImage"`
	IDBackImage  string     `json:"idBackImage"`
	FaceImage    string     `json:"faceImage"`
	// 企业认证
	CompanyName     string `json:"companyName"`
	BusinessLicense string `json:"businessLicense"`
}

// ReviewVerificationRequest 审核实名认证请求
type ReviewVerificationRequest struct {
	VerificationID string `json:"verificationId" binding:"required"`
	Action         string `json:"action" binding:"required"` // approve, reject
	RejectReason   string `json:"rejectReason"`
	ReviewerID     string `json:"reviewerId"`
}

// SetContentRatingRequest 设置内容分级请求
type SetContentRatingRequest struct {
	TenantID     string    `json:"tenantId"`
	ContentID    string    `json:"contentId" binding:"required"`
	ContentType  string    `json:"contentType" binding:"required"`
	Rating       AgeRating `json:"rating" binding:"required"`
	RatingReason string    `json:"ratingReason"`
	HasViolence  bool      `json:"hasViolence"`
	HasSexual    bool      `json:"hasSexual"`
	HasDrug      bool      `json:"hasDrug"`
	HasGambling  bool      `json:"hasGambling"`
	HasHorror    bool      `json:"hasHorror"`
	HasPolitical bool      `json:"hasPolitical"`
}

// RunComplianceCheckRequest 执行合规检查请求
type RunComplianceCheckRequest struct {
	TenantID    string `json:"tenantId"`
	ContentID   string `json:"contentId" binding:"required"`
	ContentType string `json:"contentType" binding:"required"`
	CheckType   string `json:"checkType" binding:"required"` // text, image, copyright, all
	Content     string `json:"content"`                      // 要检查的内容
}

// GenerateReportRequest 生成合规报告请求
type GenerateReportRequest struct {
	TenantID    string    `json:"tenantId"`
	ReportType  string    `json:"reportType" binding:"required"`
	PeriodStart time.Time `json:"periodStart" binding:"required"`
	PeriodEnd   time.Time `json:"periodEnd" binding:"required"`
}

// ComplianceStats 合规统计
type ComplianceStats struct {
	TenantID            string  `json:"tenantId"`
	TotalVerifications  int64   `json:"totalVerifications"`
	PendingVerifications int64  `json:"pendingVerifications"`
	ApprovedVerifications int64 `json:"approvedVerifications"`
	TotalChecks         int64   `json:"totalChecks"`
	PassedChecks        int64   `json:"passedChecks"`
	FailedChecks        int64   `json:"failedChecks"`
	ComplianceRate      float64 `json:"complianceRate"`
	TotalAlerts         int64   `json:"totalAlerts"`
	UnresolvedAlerts    int64   `json:"unresolvedAlerts"`
}
