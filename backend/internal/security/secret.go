package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	secretKeyOnce sync.Once
	secretKey     []byte
)

func getSecretKey() []byte {
	secretKeyOnce.Do(func() {
		seed := strings.TrimSpace(os.Getenv("MODEL_CREDENTIAL_SECRET"))
		if seed == "" {
			seed = strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
		}
		if seed == "" {
			seed = "agentflow_dev_model_secret_change_me"
		}
		sum := sha256.Sum256([]byte(seed))
		secretKey = sum[:]
	})
	return secretKey
}

// EncryptSecret 使用 AES-GCM 加密敏感字符串，返回包含随机 Nonce 的字节数组。
func EncryptSecret(plain string) ([]byte, error) {
	if strings.TrimSpace(plain) == "" {
		return nil, fmt.Errorf("待加密内容不能为空")
	}
	key := getSecretKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化密钥失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成随机数失败: %w", err)
	}
	cipherBytes := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return cipherBytes, nil
}

// DecryptSecret 对 EncryptSecret 生成的密文进行解密。
func DecryptSecret(ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", fmt.Errorf("密文不能为空")
	}
	key := getSecretKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化密钥失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("密文长度无效")
	}
	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	plain, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}
	return string(plain), nil
}
