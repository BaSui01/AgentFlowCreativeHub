package ai

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// CredentialProvider 定义凭证提供者接口，允许后续接入加密存储或外部密钥服务
type CredentialProvider interface {
	Get(key string) (string, error)
}

// EnvCredentialProvider 默认实现：从环境变量读取凭证
type EnvCredentialProvider struct{}

// Get 按键名读取环境变量并返回修剪后的值
func (EnvCredentialProvider) Get(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("凭证键名不能为空")
	}
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("环境变量 %s 未设置", key)
	}
	return strings.TrimSpace(value), nil
}

var (
	credentialProvider CredentialProvider = EnvCredentialProvider{}
	credentialMu       sync.RWMutex
)

// RegisterCredentialProvider 允许外部注册自定义凭证提供者
// 传入 nil 时回退到默认的环境变量实现
func RegisterCredentialProvider(provider CredentialProvider) {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	if provider == nil {
		credentialProvider = EnvCredentialProvider{}
		return
	}
	credentialProvider = provider
}

// getAPIKey 根据标识符获取 API Key
// 优先调用已注册的 CredentialProvider，失败时回退到环境变量
func getAPIKey(keyIdentifier string) string {
	if keyIdentifier == "" {
		return ""
	}

	credentialMu.RLock()
	provider := credentialProvider
	credentialMu.RUnlock()

	if provider == nil {
		provider = EnvCredentialProvider{}
	}

	if value, err := provider.Get(keyIdentifier); err == nil && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}

	// 兜底：直接读取环境变量，确保兼容旧逻辑
	if fallback, ok := os.LookupEnv(keyIdentifier); ok {
		return strings.TrimSpace(fallback)
	}

	return ""
}
