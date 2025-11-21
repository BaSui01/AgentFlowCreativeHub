package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Log      LogConfig      `mapstructure:"log"`
	AI       AIConfig       `mapstructure:"ai"`
	RAG      RagConfig      `mapstructure:"rag"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	Mode         string `mapstructure:"mode"` // debug, release, test
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	SSLMode         string `mapstructure:"sslmode"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // 秒
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Format     string `mapstructure:"format"`      // json, console
	OutputPath string `mapstructure:"output_path"` // stdout, stderr, /path/to/log
}

// AIConfig AI 模型配置
type AIConfig struct {
	OpenAI    OpenAIConfig    `mapstructure:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`
}

// RagConfig RAG 相关配置
type RagConfig struct {
	VectorStore VectorStoreConfig `mapstructure:"vector_store"`
}

// VectorStoreConfig 向量存储配置
type VectorStoreConfig struct {
	Type   string       `mapstructure:"type"`
	Qdrant QdrantConfig `mapstructure:"qdrant"`
}

// QdrantConfig Qdrant 外部向量数据库配置
type QdrantConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	APIKey          string `mapstructure:"api_key"`
	Collection      string `mapstructure:"collection"`
	VectorDimension int    `mapstructure:"vector_dimension"`
	Distance        string `mapstructure:"distance"`
	TimeoutSeconds  int    `mapstructure:"timeout_seconds"`
}

// OpenAIConfig OpenAI 配置
type OpenAIConfig struct {
	APIKey     string `mapstructure:"api_key"`
	BaseURL    string `mapstructure:"base_url"`
	OrgID      string `mapstructure:"org_id"`
	MaxRetries int    `mapstructure:"max_retries"`
}

// AnthropicConfig Anthropic 配置
type AnthropicConfig struct {
	APIKey     string `mapstructure:"api_key"`
	BaseURL    string `mapstructure:"base_url"`
	MaxRetries int    `mapstructure:"max_retries"`
}

var globalConfig *Config

// Load 加载配置
// env: 环境名称（dev, prod, test）
// configPath: 配置文件路径（可选）
func Load(env string, configPath string) (*Config, error) {
	v := viper.New()

	// 设置配置文件名和路径
	if configPath == "" {
		v.SetConfigName(env) // dev.yaml, prod.yaml
		v.AddConfigPath("./config")
		v.AddConfigPath("../config")
		v.AddConfigPath("../../config")
	} else {
		v.SetConfigFile(configPath)
	}

	v.SetConfigType("yaml")

	// 读取环境变量（优先级高于配置文件）
	v.SetEnvPrefix("APP") // 环境变量前缀：APP_
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) // 支持嵌套配置：APP_DATABASE_HOST

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get 获取全局配置
func Get() *Config {
	if globalConfig == nil {
		panic("配置未初始化，请先调用 Load()")
	}
	return globalConfig
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}
