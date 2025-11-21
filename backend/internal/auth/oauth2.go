package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

// OAuth2Provider OAuth2 提供商类型
type OAuth2Provider string

const (
	ProviderGoogle    OAuth2Provider = "google"
	ProviderGitHub    OAuth2Provider = "github"
	ProviderMicrosoft OAuth2Provider = "microsoft"
	ProviderOIDC      OAuth2Provider = "oidc" // 通用 OIDC
)

// OAuth2Config OAuth2 配置
type OAuth2Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	Endpoint     oauth2.Endpoint // 自定义端点（用于 OIDC）
}

// OAuth2Service OAuth2 服务
type OAuth2Service struct {
	configs map[OAuth2Provider]*oauth2.Config
}

// NewOAuth2Service 创建 OAuth2 服务
func NewOAuth2Service() *OAuth2Service {
	return &OAuth2Service{
		configs: make(map[OAuth2Provider]*oauth2.Config),
	}
}

// RegisterProvider 注册 OAuth2 提供商
func (s *OAuth2Service) RegisterProvider(provider OAuth2Provider, config *OAuth2Config) {
	var endpoint oauth2.Endpoint

	switch provider {
	case ProviderGoogle:
		endpoint = google.Endpoint
		if len(config.Scopes) == 0 {
			config.Scopes = []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			}
		}
	case ProviderGitHub:
		endpoint = github.Endpoint
		if len(config.Scopes) == 0 {
			config.Scopes = []string{"user:email"}
		}
	case ProviderMicrosoft:
		endpoint = microsoft.AzureADEndpoint("")
		if len(config.Scopes) == 0 {
			config.Scopes = []string{"openid", "profile", "email"}
		}
	case ProviderOIDC:
		// 使用自定义端点
		endpoint = config.Endpoint
	default:
		return
	}

	s.configs[provider] = &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Scopes:       config.Scopes,
		Endpoint:     endpoint,
	}
}

// GetAuthURL 获取授权 URL
func (s *OAuth2Service) GetAuthURL(provider OAuth2Provider, state string) (string, error) {
	config, exists := s.configs[provider]
	if !exists {
		return "", fmt.Errorf("未配置的 OAuth2 提供商: %s", provider)
	}

	return config.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

// ExchangeCode 交换授权码为访问令牌
func (s *OAuth2Service) ExchangeCode(ctx context.Context, provider OAuth2Provider, code string) (*oauth2.Token, error) {
	config, exists := s.configs[provider]
	if !exists {
		return nil, fmt.Errorf("未配置的 OAuth2 提供商: %s", provider)
	}

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("交换授权码失败: %w", err)
	}

	return token, nil
}

// GetUserInfo 获取用户信息
func (s *OAuth2Service) GetUserInfo(ctx context.Context, provider OAuth2Provider, token *oauth2.Token) (*OAuth2UserInfo, error) {
	config, exists := s.configs[provider]
	if !exists {
		return nil, fmt.Errorf("未配置的 OAuth2 提供商: %s", provider)
	}

	client := config.Client(ctx, token)

	switch provider {
	case ProviderGoogle:
		return s.getGoogleUserInfo(client)
	case ProviderGitHub:
		return s.getGitHubUserInfo(client)
	case ProviderMicrosoft:
		return s.getMicrosoftUserInfo(client)
	case ProviderOIDC:
		return s.getOIDCUserInfo(client)
	default:
		return nil, fmt.Errorf("不支持的 OAuth2 提供商: %s", provider)
	}
}

// OAuth2UserInfo OAuth2 用户信息
type OAuth2UserInfo struct {
	ID       string `json:"id"`        // 提供商用户 ID
	Email    string `json:"email"`     // 电子邮件
	Name     string `json:"name"`      // 姓名
	Picture  string `json:"picture"`   // 头像 URL
	Provider string `json:"provider"`  // 提供商
	Verified bool   `json:"verified"`  // 邮箱是否验证
}

// getGoogleUserInfo 获取 Google 用户信息
func (s *OAuth2Service) getGoogleUserInfo(client *http.Client) (*OAuth2UserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("获取 Google 用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &OAuth2UserInfo{
		ID:       data.ID,
		Email:    data.Email,
		Name:     data.Name,
		Picture:  data.Picture,
		Provider: string(ProviderGoogle),
		Verified: data.VerifiedEmail,
	}, nil
}

// getGitHubUserInfo 获取 GitHub 用户信息
func (s *OAuth2Service) getGitHubUserInfo(client *http.Client) (*OAuth2UserInfo, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("获取 GitHub 用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var userData struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return nil, fmt.Errorf("解析用户数据失败: %w", err)
	}

	// 获取邮箱
	emailResp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return nil, fmt.Errorf("获取 GitHub 邮箱失败: %w", err)
	}
	defer emailResp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(emailResp.Body).Decode(&emails); err != nil {
		return nil, fmt.Errorf("解析邮箱数据失败: %w", err)
	}

	// 查找主邮箱
	var primaryEmail string
	var verified bool
	for _, e := range emails {
		if e.Primary {
			primaryEmail = e.Email
			verified = e.Verified
			break
		}
	}

	if primaryEmail == "" && len(emails) > 0 {
		primaryEmail = emails[0].Email
		verified = emails[0].Verified
	}

	name := userData.Name
	if name == "" {
		name = userData.Login
	}

	return &OAuth2UserInfo{
		ID:       fmt.Sprintf("%d", userData.ID),
		Email:    primaryEmail,
		Name:     name,
		Picture:  userData.AvatarURL,
		Provider: string(ProviderGitHub),
		Verified: verified,
	}, nil
}

// getMicrosoftUserInfo 获取 Microsoft 用户信息
func (s *OAuth2Service) getMicrosoftUserInfo(client *http.Client) (*OAuth2UserInfo, error) {
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return nil, fmt.Errorf("获取 Microsoft 用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		ID                string `json:"id"`
		UserPrincipalName string `json:"userPrincipalName"`
		DisplayName       string `json:"displayName"`
		Mail              string `json:"mail"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	email := data.Mail
	if email == "" {
		email = data.UserPrincipalName
	}

	return &OAuth2UserInfo{
		ID:       data.ID,
		Email:    email,
		Name:     data.DisplayName,
		Provider: string(ProviderMicrosoft),
		Verified: true, // Microsoft 账户默认已验证
	}, nil
}

// getOIDCUserInfo 获取 OIDC 用户信息
func (s *OAuth2Service) getOIDCUserInfo(client *http.Client) (*OAuth2UserInfo, error) {
	// OIDC 标准 UserInfo 端点
	resp, err := client.Get("https://your-oidc-provider.com/userinfo")
	if err != nil {
		return nil, fmt.Errorf("获取 OIDC 用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var data struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified bool   `json:"email_verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &OAuth2UserInfo{
		ID:       data.Sub,
		Email:    data.Email,
		Name:     data.Name,
		Picture:  data.Picture,
		Provider: string(ProviderOIDC),
		Verified: data.EmailVerified,
	}, nil
}
