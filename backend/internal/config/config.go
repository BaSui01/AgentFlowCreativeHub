package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Log       LogConfig       `mapstructure:"log"`
	AI        AIConfig        `mapstructure:"ai"`
	RAG       RagConfig       `mapstructure:"rag"`
	Workspace WorkspaceConfig `mapstructure:"workspace"`
	Cache     CacheConfig     `mapstructure:"cache"` // 新增:缓存配置
}

// WorkspaceConfig 工作区文件系统配置
type WorkspaceConfig struct {
	BasePath        string `mapstructure:"base_path"`         // 工作区根目录，默认 ./workspace
	EnableDiskStore bool   `mapstructure:"enable_disk_store"` // 是否启用磁盘存储
	MaxFileSize     int64  `mapstructure:"max_file_size"`     // 单文件大小限制（字节），默认 10MB
	Artifact        ArtifactConfig `mapstructure:"artifact"`
}

// ArtifactConfig 智能体产出文件配置
type ArtifactConfig struct {
	NamingPattern   string `mapstructure:"naming_pattern"`   // 命名模式: {agent}-{type}-{timestamp}-{seq}
	OrganizeByAgent bool   `mapstructure:"organize_by_agent"` // 按智能体分目录
	OrganizeBySession bool `mapstructure:"organize_by_session"` // 按会话分目录
	RetentionDays   int    `mapstructure:"retention_days"`   // 保留天数，0表示永久
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
	AutoMigrate     bool   `mapstructure:"auto_migrate"`      // 是否自动迁移表结构
}

// RedisConfig Redis 配置
type RedisConfig struct {
	// 连接模式: standalone(单节点), sentinel(哨兵), cluster(集群)
	Mode string `mapstructure:"mode"`

	// 单节点模式配置
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`

	// 哨兵模式配置
	MasterName       string   `mapstructure:"master_name"`        // 主节点名称
	SentinelAddrs    []string `mapstructure:"sentinel_addrs"`     // 哨兵地址列表
	SentinelPassword string   `mapstructure:"sentinel_password"`  // 哨兵密码（可选）

	// 集群模式配置
	ClusterAddrs []string `mapstructure:"cluster_addrs"` // 集群节点地址列表

	// 通用配置
	PoolSize     int `mapstructure:"pool_size"`      // 连接池大小
	MinIdleConns int `mapstructure:"min_idle_conns"` // 最小空闲连接数
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

// CacheConfig 缓存配置
type CacheConfig struct {
	Disk DiskCacheConfig `mapstructure:"disk"` // 硬盘缓存(L3层)
}

// DiskCacheConfig 硬盘缓存配置
type DiskCacheConfig struct {
	Enabled         bool   `mapstructure:"enabled"`          // 是否启用硬盘缓存
	DBPath          string `mapstructure:"db_path"`          // 数据库文件路径
	MaxSizeGB       int    `mapstructure:"max_size_gb"`      // 最大缓存大小(GB)
	TTL             string `mapstructure:"ttl"`              // 缓存过期时间(如"720h"表示30天)
	CleanupInterval string `mapstructure:"cleanup_interval"` // 清理间隔(如"30m")
	MonitorInterval string `mapstructure:"monitor_interval"` // 监控日志输出间隔(如"5m")
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
