package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DingTalkOAuthProvider 钉钉 OAuth2 提供者
type DingTalkOAuthProvider struct {
	appKey      string
	appSecret   string
	redirectURI string
	httpClient  *http.Client
}

// DingTalkOAuthConfig 钉钉 OAuth 配置
type DingTalkOAuthConfig struct {
	AppKey      string // 也叫 Client ID
	AppSecret   string // 也叫 Client Secret
	RedirectURI string
}

// NewDingTalkOAuthProvider 创建钉钉 OAuth 提供者
func NewDingTalkOAuthProvider(config *DingTalkOAuthConfig) *DingTalkOAuthProvider {
	return &DingTalkOAuthProvider{
		appKey:      config.AppKey,
		appSecret:   config.AppSecret,
		redirectURI: config.RedirectURI,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetAuthURL 获取授权 URL (扫码登录)
func (p *DingTalkOAuthProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("redirect_uri", p.redirectURI)
	params.Set("response_type", "code")
	params.Set("client_id", p.appKey)
	params.Set("scope", "openid")
	params.Set("state", state)
	params.Set("prompt", "consent")

	return "https://login.dingtalk.com/oauth2/auth?" + params.Encode()
}

// GetQRCodeURL 获取扫码登录二维码 URL (旧版)
func (p *DingTalkOAuthProvider) GetQRCodeURL(state string) string {
	params := url.Values{}
	params.Set("appid", p.appKey)
	params.Set("response_type", "code")
	params.Set("scope", "snsapi_login")
	params.Set("state", state)
	params.Set("redirect_uri", p.redirectURI)

	return fmt.Sprintf("https://oapi.dingtalk.com/connect/qrconnect?%s", params.Encode())
}

// sign 生成签名
func (p *DingTalkOAuthProvider) sign(timestamp string) string {
	h := hmac.New(sha256.New, []byte(p.appSecret))
	h.Write([]byte(timestamp))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// DingTalkAccessToken 钉钉访问令牌
type DingTalkAccessToken struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expireIn"`
	CorpID       string `json:"corpId,omitempty"`
}

// DingTalkErrorResponse 钉钉错误响应
type DingTalkErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ExchangeToken 交换访问令牌 (新版 OAuth 2.0)
func (p *DingTalkOAuthProvider) ExchangeToken(ctx context.Context, code string) (*DingTalkAccessToken, error) {
	reqBody := map[string]string{
		"clientId":     p.appKey,
		"clientSecret": p.appSecret,
		"code":         code,
		"grantType":    "authorization_code",
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.dingtalk.com/v1.0/oauth2/userAccessToken",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp DingTalkErrorResponse
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("dingtalk error %s: %s", errResp.Code, errResp.Message)
	}

	var token DingTalkAccessToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &token, nil
}

// DingTalkUserInfo 钉钉用户信息
type DingTalkUserInfo struct {
	Nick      string `json:"nick"`
	UnionID   string `json:"unionId"`
	OpenID    string `json:"openId"`
	AvatarURL string `json:"avatarUrl"`
	Mobile    string `json:"mobile,omitempty"`
	Email     string `json:"email,omitempty"`
	StateCode string `json:"stateCode,omitempty"` // 手机号国家码
}

// GetUserInfo 获取用户信息
func (p *DingTalkOAuthProvider) GetUserInfo(ctx context.Context, accessToken string) (*DingTalkUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		"https://api.dingtalk.com/v1.0/contact/users/me", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-acs-dingtalk-access-token", accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp DingTalkErrorResponse
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("dingtalk error %s: %s", errResp.Code, errResp.Message)
	}

	var userInfo DingTalkUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &userInfo, nil
}

// GetUserInfoByCode 通过临时授权码获取用户信息 (旧版 SNS)
func (p *DingTalkOAuthProvider) GetUserInfoByCode(ctx context.Context, code string) (*DingTalkSNSUserInfo, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signature := p.sign(timestamp)

	params := url.Values{}
	params.Set("accessKey", p.appKey)
	params.Set("timestamp", timestamp)
	params.Set("signature", signature)

	reqBody := map[string]string{
		"tmp_auth_code": code,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://oapi.dingtalk.com/sns/getuserinfo_bycode?"+params.Encode(),
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result struct {
		ErrCode  int                  `json:"errcode"`
		ErrMsg   string               `json:"errmsg"`
		UserInfo DingTalkSNSUserInfo `json:"user_info"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.ErrCode != 0 {
		return nil, fmt.Errorf("dingtalk error %d: %s", result.ErrCode, result.ErrMsg)
	}

	return &result.UserInfo, nil
}

// DingTalkSNSUserInfo 钉钉 SNS 用户信息 (旧版)
type DingTalkSNSUserInfo struct {
	Nick    string `json:"nick"`
	OpenID  string `json:"openid"`
	UnionID string `json:"unionid"`
}

// GetProviderName 获取提供者名称
func (p *DingTalkOAuthProvider) GetProviderName() string {
	return "dingtalk"
}

// DingTalkOAuthIdentity 钉钉 OAuth 身份
type DingTalkOAuthIdentity struct {
	Provider   string                 `json:"provider"`
	ProviderID string                 `json:"providerId"`
	Email      string                 `json:"email,omitempty"`
	Name       string                 `json:"name"`
	Avatar     string                 `json:"avatar,omitempty"`
	Raw        map[string]interface{} `json:"raw,omitempty"`
}

// ToOAuthIdentity 转换为通用身份
func (u *DingTalkUserInfo) ToOAuthIdentity() *DingTalkOAuthIdentity {
	return &DingTalkOAuthIdentity{
		Provider:   "dingtalk",
		ProviderID: u.OpenID,
		Email:      u.Email,
		Name:       u.Nick,
		Avatar:     u.AvatarURL,
		Raw: map[string]interface{}{
			"openId":  u.OpenID,
			"unionId": u.UnionID,
			"nick":    u.Nick,
			"mobile":  u.Mobile,
		},
	}
}

// ToOAuthIdentity 转换为通用身份 (SNS 版本)
func (u *DingTalkSNSUserInfo) ToOAuthIdentity() *DingTalkOAuthIdentity {
	return &DingTalkOAuthIdentity{
		Provider:   "dingtalk",
		ProviderID: u.OpenID,
		Name:       u.Nick,
		Raw: map[string]interface{}{
			"openid":  u.OpenID,
			"unionid": u.UnionID,
			"nick":    u.Nick,
		},
	}
}
