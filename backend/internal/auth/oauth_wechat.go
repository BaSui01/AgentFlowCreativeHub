package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WeChatOAuthProvider 微信 OAuth2 提供者
type WeChatOAuthProvider struct {
	appID       string
	appSecret   string
	redirectURI string
	httpClient  *http.Client
	scopes      []string
}

// WeChatOAuthConfig 微信 OAuth 配置
type WeChatOAuthConfig struct {
	AppID       string
	AppSecret   string
	RedirectURI string
	Scopes      []string // snsapi_base, snsapi_userinfo
}

// NewWeChatOAuthProvider 创建微信 OAuth 提供者
func NewWeChatOAuthProvider(config *WeChatOAuthConfig) *WeChatOAuthProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"snsapi_userinfo"}
	}
	return &WeChatOAuthProvider{
		appID:       config.AppID,
		appSecret:   config.AppSecret,
		redirectURI: config.RedirectURI,
		scopes:      config.Scopes,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetAuthURL 获取授权 URL
func (p *WeChatOAuthProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("appid", p.appID)
	params.Set("redirect_uri", p.redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", p.scopes[0])
	params.Set("state", state)

	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + params.Encode() + "#wechat_redirect"
}

// GetQRCodeAuthURL 获取扫码登录 URL (PC 端)
func (p *WeChatOAuthProvider) GetQRCodeAuthURL(state string) string {
	params := url.Values{}
	params.Set("appid", p.appID)
	params.Set("redirect_uri", p.redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "snsapi_login")
	params.Set("state", state)

	return "https://open.weixin.qq.com/connect/qrconnect?" + params.Encode() + "#wechat_redirect"
}

// WeChatAccessToken 微信访问令牌
type WeChatAccessToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid,omitempty"`
	ErrCode      int    `json:"errcode,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
}

// ExchangeToken 交换访问令牌
func (p *WeChatOAuthProvider) ExchangeToken(ctx context.Context, code string) (*WeChatAccessToken, error) {
	params := url.Values{}
	params.Set("appid", p.appID)
	params.Set("secret", p.appSecret)
	params.Set("code", code)
	params.Set("grant_type", "authorization_code")

	url := "https://api.weixin.qq.com/sns/oauth2/access_token?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var token WeChatAccessToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if token.ErrCode != 0 {
		return nil, fmt.Errorf("wechat error %d: %s", token.ErrCode, token.ErrMsg)
	}

	return &token, nil
}

// WeChatUserInfo 微信用户信息
type WeChatUserInfo struct {
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"` // 1: 男, 2: 女, 0: 未知
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid,omitempty"`
	ErrCode    int      `json:"errcode,omitempty"`
	ErrMsg     string   `json:"errmsg,omitempty"`
}

// GetUserInfo 获取用户信息
func (p *WeChatOAuthProvider) GetUserInfo(ctx context.Context, accessToken, openID string) (*WeChatUserInfo, error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("openid", openID)
	params.Set("lang", "zh_CN")

	url := "https://api.weixin.qq.com/sns/userinfo?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var userInfo WeChatUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if userInfo.ErrCode != 0 {
		return nil, fmt.Errorf("wechat error %d: %s", userInfo.ErrCode, userInfo.ErrMsg)
	}

	return &userInfo, nil
}

// RefreshToken 刷新访问令牌
func (p *WeChatOAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*WeChatAccessToken, error) {
	params := url.Values{}
	params.Set("appid", p.appID)
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", refreshToken)

	url := "https://api.weixin.qq.com/sns/oauth2/refresh_token?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var token WeChatAccessToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if token.ErrCode != 0 {
		return nil, fmt.Errorf("wechat error %d: %s", token.ErrCode, token.ErrMsg)
	}

	return &token, nil
}

// ValidateToken 验证访问令牌
func (p *WeChatOAuthProvider) ValidateToken(ctx context.Context, accessToken, openID string) (bool, error) {
	params := url.Values{}
	params.Set("access_token", accessToken)
	params.Set("openid", openID)

	url := "https://api.weixin.qq.com/sns/auth?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}

	return result.ErrCode == 0, nil
}

// GetProviderName 获取提供者名称
func (p *WeChatOAuthProvider) GetProviderName() string {
	return "wechat"
}

// ToOAuthIdentity 转换为通用身份
func (u *WeChatUserInfo) ToOAuthIdentity() *WeChatOAuthIdentity {
	gender := ""
	switch u.Sex {
	case 1:
		gender = "male"
	case 2:
		gender = "female"
	}

	return &WeChatOAuthIdentity{
		Provider:   "wechat",
		ProviderID: u.OpenID,
		Email:      "", // 微信不提供邮箱
		Name:       u.Nickname,
		Avatar:     u.HeadImgURL,
		Raw: map[string]interface{}{
			"openid":   u.OpenID,
			"unionid":  u.UnionID,
			"nickname": u.Nickname,
			"sex":      gender,
			"province": u.Province,
			"city":     u.City,
			"country":  u.Country,
		},
	}
}

// WeChatOAuthIdentity 微信 OAuth 身份
type WeChatOAuthIdentity struct {
	Provider   string                 `json:"provider"`
	ProviderID string                 `json:"providerId"`
	Email      string                 `json:"email,omitempty"`
	Name       string                 `json:"name"`
	Avatar     string                 `json:"avatar,omitempty"`
	Raw        map[string]interface{} `json:"raw,omitempty"`
}
