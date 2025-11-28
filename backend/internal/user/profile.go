package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ProfileService 用户资料服务
type ProfileService struct {
	store ProfileStore
}

// ProfileStore 资料存储接口
type ProfileStore interface {
	GetProfile(ctx context.Context, userID string) (*UserProfile, error)
	UpdateProfile(ctx context.Context, profile *UserProfile) error
	GetPreferences(ctx context.Context, userID string) (*UserPreferences, error)
	UpdatePreferences(ctx context.Context, userID string, prefs *UserPreferences) error
	GetActivity(ctx context.Context, userID string, days int) (*ActivityStats, error)
}

// UserProfile 用户资料
type UserProfile struct {
	UserID      string         `json:"user_id"`
	Nickname    string         `json:"nickname"`
	Avatar      string         `json:"avatar,omitempty"`
	Bio         string         `json:"bio,omitempty"`
	Location    string         `json:"location,omitempty"`
	Website     string         `json:"website,omitempty"`
	Company     string         `json:"company,omitempty"`
	JobTitle    string         `json:"job_title,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Social      SocialLinks    `json:"social,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// SocialLinks 社交链接
type SocialLinks struct {
	Twitter  string `json:"twitter,omitempty"`
	GitHub   string `json:"github,omitempty"`
	LinkedIn string `json:"linkedin,omitempty"`
	WeChat   string `json:"wechat,omitempty"`
}

// UserPreferences 用户偏好设置
type UserPreferences struct {
	UserID      string              `json:"user_id"`
	Theme       string              `json:"theme"`       // light / dark / system
	Language    string              `json:"language"`    // zh-CN / en-US
	Timezone    string              `json:"timezone"`    // Asia/Shanghai
	DateFormat  string              `json:"date_format"` // YYYY-MM-DD
	TimeFormat  string              `json:"time_format"` // 24h / 12h
	Editor      EditorPrefs         `json:"editor"`
	AI          AIPrefs             `json:"ai"`
	Notify      NotificationPrefs   `json:"notify"`
	Privacy     PrivacyPrefs        `json:"privacy"`
	Extra       map[string]any      `json:"extra,omitempty"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// EditorPrefs 编辑器偏好
type EditorPrefs struct {
	FontFamily   string `json:"font_family"`
	FontSize     int    `json:"font_size"`
	LineHeight   float64 `json:"line_height"`
	TabSize      int    `json:"tab_size"`
	WordWrap     bool   `json:"word_wrap"`
	AutoSave     bool   `json:"auto_save"`
	SpellCheck   bool   `json:"spell_check"`
	VimMode      bool   `json:"vim_mode"`
}

// AIPrefs AI 偏好
type AIPrefs struct {
	DefaultModel    string  `json:"default_model"`
	Temperature     float64 `json:"temperature"`
	MaxTokens       int     `json:"max_tokens"`
	StreamResponse  bool    `json:"stream_response"`
	ShowTokenCount  bool    `json:"show_token_count"`
	AutoSuggest     bool    `json:"auto_suggest"`
}

// NotificationPrefs 通知偏好
type NotificationPrefs struct {
	Email       bool `json:"email"`
	Push        bool `json:"push"`
	InApp       bool `json:"in_app"`
	Digest      bool `json:"digest"`       // 每日摘要
	Marketing   bool `json:"marketing"`    // 营销通知
	AgentDone   bool `json:"agent_done"`   // Agent 完成通知
	WorkflowErr bool `json:"workflow_err"` // 工作流错误通知
}

// PrivacyPrefs 隐私偏好
type PrivacyPrefs struct {
	ProfilePublic   bool `json:"profile_public"`
	ShowActivity    bool `json:"show_activity"`
	ShowEmail       bool `json:"show_email"`
	DataCollection  bool `json:"data_collection"`
	AnalyticsCookie bool `json:"analytics_cookie"`
}

// ActivityStats 活跃度统计
type ActivityStats struct {
	UserID        string           `json:"user_id"`
	TotalLogins   int64            `json:"total_logins"`
	LastLogin     *time.Time       `json:"last_login,omitempty"`
	TotalActions  int64            `json:"total_actions"`
	DailyStats    []DailyActivity  `json:"daily_stats"`
	TopActions    []ActionCount    `json:"top_actions"`
	ActiveDays    int              `json:"active_days"`
	CurrentStreak int              `json:"current_streak"`
	LongestStreak int              `json:"longest_streak"`
}

// DailyActivity 每日活动
type DailyActivity struct {
	Date    string `json:"date"`
	Actions int64  `json:"actions"`
	Logins  int64  `json:"logins"`
}

// ActionCount 操作统计
type ActionCount struct {
	Action string `json:"action"`
	Count  int64  `json:"count"`
}

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrInvalidAvatar   = errors.New("invalid avatar url")
)

// NewProfileService 创建资料服务
func NewProfileService(store ProfileStore) *ProfileService {
	return &ProfileService{store: store}
}

// GetProfile 获取用户资料
func (s *ProfileService) GetProfile(ctx context.Context, userID string) (*UserProfile, error) {
	return s.store.GetProfile(ctx, userID)
}

// UpdateProfile 更新用户资料
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *UserProfile) error {
	// 验证 Avatar URL
	if profile.Avatar != "" && !isValidURL(profile.Avatar) {
		return ErrInvalidAvatar
	}

	// 验证网站 URL
	if profile.Website != "" && !isValidURL(profile.Website) {
		return fmt.Errorf("invalid website url")
	}

	profile.UpdatedAt = time.Now()
	return s.store.UpdateProfile(ctx, profile)
}

// UpdateAvatar 更新头像
func (s *ProfileService) UpdateAvatar(ctx context.Context, userID, avatarURL string) error {
	if !isValidURL(avatarURL) {
		return ErrInvalidAvatar
	}

	profile, err := s.store.GetProfile(ctx, userID)
	if err != nil {
		// 创建新资料
		profile = &UserProfile{
			UserID:    userID,
			Avatar:    avatarURL,
			CreatedAt: time.Now(),
		}
	} else {
		profile.Avatar = avatarURL
	}

	return s.UpdateProfile(ctx, profile)
}

// GetPreferences 获取偏好设置
func (s *ProfileService) GetPreferences(ctx context.Context, userID string) (*UserPreferences, error) {
	prefs, err := s.store.GetPreferences(ctx, userID)
	if err != nil {
		// 返回默认设置
		return s.defaultPreferences(userID), nil
	}
	return prefs, nil
}

// UpdatePreferences 更新偏好设置
func (s *ProfileService) UpdatePreferences(ctx context.Context, userID string, prefs *UserPreferences) error {
	prefs.UserID = userID
	prefs.UpdatedAt = time.Now()
	return s.store.UpdatePreferences(ctx, userID, prefs)
}

// PatchPreferences 部分更新偏好（合并更新）
func (s *ProfileService) PatchPreferences(ctx context.Context, userID string, patch map[string]any) error {
	current, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return err
	}

	// 转为 JSON 再合并
	currentJSON, _ := json.Marshal(current)
	var currentMap map[string]any
	json.Unmarshal(currentJSON, &currentMap)

	// 递归合并
	merged := mergeMaps(currentMap, patch)

	// 转回结构体
	mergedJSON, _ := json.Marshal(merged)
	var updated UserPreferences
	if err := json.Unmarshal(mergedJSON, &updated); err != nil {
		return fmt.Errorf("invalid patch: %w", err)
	}

	return s.UpdatePreferences(ctx, userID, &updated)
}

// GetActivity 获取活跃度统计
func (s *ProfileService) GetActivity(ctx context.Context, userID string, days int) (*ActivityStats, error) {
	if days <= 0 {
		days = 30
	}
	return s.store.GetActivity(ctx, userID, days)
}

// defaultPreferences 默认偏好设置
func (s *ProfileService) defaultPreferences(userID string) *UserPreferences {
	return &UserPreferences{
		UserID:     userID,
		Theme:      "system",
		Language:   "zh-CN",
		Timezone:   "Asia/Shanghai",
		DateFormat: "YYYY-MM-DD",
		TimeFormat: "24h",
		Editor: EditorPrefs{
			FontFamily: "system-ui",
			FontSize:   14,
			LineHeight: 1.6,
			TabSize:    2,
			WordWrap:   true,
			AutoSave:   true,
			SpellCheck: false,
			VimMode:    false,
		},
		AI: AIPrefs{
			DefaultModel:   "gpt-4",
			Temperature:    0.7,
			MaxTokens:      2048,
			StreamResponse: true,
			ShowTokenCount: false,
			AutoSuggest:    true,
		},
		Notify: NotificationPrefs{
			Email:       true,
			Push:        true,
			InApp:       true,
			Digest:      false,
			Marketing:   false,
			AgentDone:   true,
			WorkflowErr: true,
		},
		Privacy: PrivacyPrefs{
			ProfilePublic:   false,
			ShowActivity:    false,
			ShowEmail:       false,
			DataCollection:  true,
			AnalyticsCookie: true,
		},
	}
}

func isValidURL(s string) bool {
	if s == "" {
		return false
	}
	return len(s) > 8 && (s[:7] == "http://" || s[:8] == "https://")
}

func mergeMaps(base, patch map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range patch {
		if baseMap, ok := result[k].(map[string]any); ok {
			if patchMap, ok := v.(map[string]any); ok {
				result[k] = mergeMaps(baseMap, patchMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// ExportProfile 导出用户资料（GDPR）
func (s *ProfileService) ExportProfile(ctx context.Context, userID string) ([]byte, error) {
	profile, _ := s.GetProfile(ctx, userID)
	prefs, _ := s.GetPreferences(ctx, userID)
	activity, _ := s.GetActivity(ctx, userID, 365)

	export := map[string]any{
		"export_date":  time.Now(),
		"profile":      profile,
		"preferences":  prefs,
		"activity":     activity,
	}

	return json.MarshalIndent(export, "", "  ")
}
