// Package cache 提供缓存相关功能
package cache

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DiskCache 硬盘缓存管理器
type DiskCache struct {
	db      *sql.DB
	dbPath  string
	ttl     time.Duration
	maxSize int64 // 最大缓存大小 (字节)
}

// CacheEntry 缓存条目
type CacheEntry struct {
	CacheKey       string          `json:"cache_key"`
	Model          string          `json:"model"`
	PromptHash     string          `json:"prompt_hash"`
	Response       string          `json:"response"`
	TokensUsed     int             `json:"tokens_used"`
	CostUSD        float64         `json:"cost_usd"`
	HitCount       int             `json:"hit_count"`
	CreatedAt      time.Time       `json:"created_at"`
	LastAccessedAt time.Time       `json:"last_accessed_at"`
	ExpiresAt      *time.Time      `json:"expires_at,omitempty"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

// NewDiskCache 创建硬盘缓存实例
func NewDiskCache(dbPath string, ttl time.Duration, maxSizeGB int) (*DiskCache, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 设置连接池参数以提升并发性能
	db.SetMaxOpenConns(10)  // 最大打开连接数
	db.SetMaxIdleConns(5)   // 最大空闲连接数

	// 性能优化设置
	pragmas := []string{
		"PRAGMA journal_mode=WAL",           // 写前日志模式,提升并发性能
		"PRAGMA synchronous=NORMAL",         // 正常同步模式,平衡性能和安全
		"PRAGMA cache_size=-64000",          // 64MB 缓存
		"PRAGMA temp_store=MEMORY",          // 临时表存储在内存
		"PRAGMA mmap_size=268435456",        // 256MB 内存映射
		"PRAGMA busy_timeout=10000",         // 10秒忙等待超时 (增加以支持高并发)
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("设置数据库参数失败 [%s]: %w", pragma, err)
		}
	}

	// 创建表结构
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	cache := &DiskCache{
		db:      db,
		dbPath:  dbPath,
		ttl:     ttl,
		maxSize: int64(maxSizeGB) * 1024 * 1024 * 1024,
	}

	// 启动后台清理任务
	go cache.cleanupLoop(context.Background())

	return cache, nil
}

// initSchema 初始化数据库表结构
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS llm_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT NOT NULL UNIQUE,
		model TEXT NOT NULL,
		prompt_hash TEXT NOT NULL,
		prompt TEXT,
		response TEXT NOT NULL,
		tokens_used INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		hit_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME,
		compressed BOOLEAN DEFAULT 0,
		metadata JSON
	);

	CREATE INDEX IF NOT EXISTS idx_cache_key ON llm_cache(cache_key);
	CREATE INDEX IF NOT EXISTS idx_model_prompt ON llm_cache(model, prompt_hash);
	CREATE INDEX IF NOT EXISTS idx_expires_at ON llm_cache(expires_at);
	CREATE INDEX IF NOT EXISTS idx_last_accessed ON llm_cache(last_accessed_at);

	CREATE TABLE IF NOT EXISTS cache_stats (
		date DATE PRIMARY KEY,
		total_hits INTEGER DEFAULT 0,
		total_misses INTEGER DEFAULT 0,
		total_saves_usd REAL DEFAULT 0.0,
		avg_response_time_ms INTEGER DEFAULT 0
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("初始化数据库表结构失败: %w", err)
	}

	return nil
}

// GenerateCacheKey 生成缓存键
func GenerateCacheKey(model, prompt string) string {
	data := fmt.Sprintf("%s:%s", model, prompt)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Get 读取缓存
func (c *DiskCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	query := `
		SELECT cache_key, model, prompt_hash, response, tokens_used, cost_usd,
		       hit_count, created_at, last_accessed_at, expires_at, metadata
		FROM llm_cache
		WHERE cache_key = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
	`

	var entry CacheEntry
	var expiresAt sql.NullTime
	var metadata sql.NullString

	err := c.db.QueryRowContext(ctx, query, key).Scan(
		&entry.CacheKey,
		&entry.Model,
		&entry.PromptHash,
		&entry.Response,
		&entry.TokensUsed,
		&entry.CostUSD,
		&entry.HitCount,
		&entry.CreatedAt,
		&entry.LastAccessedAt,
		&expiresAt,
		&metadata,
	)

	if err == sql.ErrNoRows {
		return nil, nil // 缓存未命中
	}
	if err != nil {
		return nil, fmt.Errorf("查询缓存失败: %w", err)
	}

	if expiresAt.Valid {
		entry.ExpiresAt = &expiresAt.Time
	}
	if metadata.Valid {
		entry.Metadata = json.RawMessage(metadata.String)
	}

	// 更新访问统计 (同步执行以确保测试一致性)
	c.incrementHitCount(key)

	return &entry, nil
}

// Set 写入缓存
func (c *DiskCache) Set(ctx context.Context, entry *CacheEntry) error {
	expiresAt := sql.NullTime{}
	if entry.ExpiresAt != nil {
		expiresAt.Valid = true
		expiresAt.Time = *entry.ExpiresAt
	} else if c.ttl > 0 {
		expiresAt.Valid = true
		expiresAt.Time = time.Now().Add(c.ttl)
	}

	metadata := sql.NullString{}
	if len(entry.Metadata) > 0 {
		metadata.Valid = true
		metadata.String = string(entry.Metadata)
	}

	query := `
		INSERT INTO llm_cache (
			cache_key, model, prompt_hash, response, tokens_used, cost_usd,
			expires_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			response = excluded.response,
			tokens_used = excluded.tokens_used,
			cost_usd = excluded.cost_usd,
			updated_at = CURRENT_TIMESTAMP,
			metadata = excluded.metadata
	`

	_, err := c.db.ExecContext(ctx, query,
		entry.CacheKey,
		entry.Model,
		entry.PromptHash,
		entry.Response,
		entry.TokensUsed,
		entry.CostUSD,
		expiresAt,
		metadata,
	)

	if err != nil {
		return fmt.Errorf("写入缓存失败: %w", err)
	}

	// 检查缓存大小并清理
	go c.checkAndCleanup()

	return nil
}

// incrementHitCount 增加命中计数
// 注意：此方法是同步执行的，测试时需要考虑这一点
func (c *DiskCache) incrementHitCount(key string) {
	query := `
		UPDATE llm_cache
		SET hit_count = hit_count + 1,
		    last_accessed_at = CURRENT_TIMESTAMP
		WHERE cache_key = ?
	`
	// 移除异步执行，改为同步，确保测试时数据一致性
	_, _ = c.db.Exec(query, key)
}

// Delete 删除缓存
func (c *DiskCache) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM llm_cache WHERE cache_key = ?`
	_, err := c.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("删除缓存失败: %w", err)
	}
	return nil
}

// Clear 清空所有缓存
func (c *DiskCache) Clear(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, "DELETE FROM llm_cache")
	if err != nil {
		return fmt.Errorf("清空缓存失败: %w", err)
	}
	return nil
}

// cleanupLoop 定期清理过期缓存
func (c *DiskCache) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

// cleanup 执行清理操作
func (c *DiskCache) cleanup() {
	// 1. 删除过期条目
	result, err := c.db.Exec(`
		DELETE FROM llm_cache
		WHERE expires_at IS NOT NULL AND expires_at < datetime('now')
	`)
	if err == nil {
		if rows, _ := result.RowsAffected(); rows > 0 {
			fmt.Printf("清理过期缓存: 删除 %d 条记录\n", rows)
		}
	}

	// 2. 执行 VACUUM 压缩数据库
	c.db.Exec("VACUUM")
}

// checkAndCleanup 检查缓存大小并清理
func (c *DiskCache) checkAndCleanup() {
	var totalSize int64
	err := c.db.QueryRow(`
		SELECT COALESCE(SUM(length(response)), 0)
		FROM llm_cache
	`).Scan(&totalSize)

	if err != nil || totalSize < c.maxSize {
		return
	}

	// 使用 LRU 策略删除最旧的 10%
	result, err := c.db.Exec(`
		DELETE FROM llm_cache
		WHERE id IN (
			SELECT id FROM llm_cache
			ORDER BY last_accessed_at ASC
			LIMIT (SELECT COUNT(*) / 10 FROM llm_cache)
		)
	`)

	if err == nil {
		if rows, _ := result.RowsAffected(); rows > 0 {
			fmt.Printf("LRU 淘汰: 删除 %d 条记录 (缓存大小: %.2f MB / %.2f MB)\n",
				rows, float64(totalSize)/1024/1024, float64(c.maxSize)/1024/1024)
		}
	}
}

// GetStats 获取缓存统计
func (c *DiskCache) GetStats(ctx context.Context) (map[string]any, error) {
	var stats struct {
		TotalEntries int
		TotalHits    int64
		TotalSizeKB  int64
		AvgHitCount  float64
		OldestEntry  sql.NullString
		NewestEntry  sql.NullString
	}

	query := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(hit_count), 0) as total_hits,
			COALESCE(SUM(length(response))/1024, 0) as total_size_kb,
			COALESCE(AVG(hit_count), 0) as avg_hit_count,
			MIN(created_at) as oldest,
			MAX(created_at) as newest
		FROM llm_cache
	`

	err := c.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalEntries,
		&stats.TotalHits,
		&stats.TotalSizeKB,
		&stats.AvgHitCount,
		&stats.OldestEntry,
		&stats.NewestEntry,
	)

	if err != nil {
		return nil, fmt.Errorf("获取统计数据失败: %w", err)
	}

	result := map[string]any{
		"total_entries": stats.TotalEntries,
		"total_hits":    stats.TotalHits,
		"total_size_mb": float64(stats.TotalSizeKB) / 1024,
		"avg_hit_count": stats.AvgHitCount,
	}

	if stats.OldestEntry.Valid && stats.OldestEntry.String != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", stats.OldestEntry.String); err == nil {
			result["oldest_entry"] = t
		}
	}
	if stats.NewestEntry.Valid && stats.NewestEntry.String != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", stats.NewestEntry.String); err == nil {
			result["newest_entry"] = t
		}
	}

	return result, nil
}

// Close 关闭数据库连接
func (c *DiskCache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}
